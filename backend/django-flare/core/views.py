import json
import hmac
import os
import uuid
from django.db import transaction
from django.utils import timezone
from django.utils.dateparse import parse_datetime
from django.http import JsonResponse
from rest_framework.decorators import api_view
from rest_framework.exceptions import APIException, ValidationError
from .models import OfflineSyncQueue, SyncEvent, SyncConflict

class EventIDReuse(APIException):
    status_code = 409
    default_detail = 'event_id_reused'
    default_code = 'event_id_reused'

@api_view(['GET'])
def health(request):
    return JsonResponse({'status': 'ok'})

@api_view(['POST'])
def queue_item(request):
    data = request.data if hasattr(request, 'data') else json.loads(request.body.decode('utf-8'))
    item = OfflineSyncQueue.objects.create(
        entity_type=data.get('entity_type', 'generic'),
        payload_json=json.dumps(data.get('payload', {})),
    )
    return JsonResponse({'queue_id': item.queue_id}, status=201)

@api_view(['GET'])
def pull_queue(request):
    limit = int(request.GET.get('limit', 10))
    items = OfflineSyncQueue.objects.filter(status=OfflineSyncQueue.Status.PENDING)[:limit]
    res = []
    for it in items:
        res.append({
            'queue_id': it.queue_id,
            'entity_type': it.entity_type,
            'payload': json.loads(it.payload_json),
            'created_at': it.created_at.isoformat(),
        })
    return JsonResponse({'items': res})

@api_view(['POST'])
def sync_push(request):
    content_length = request.headers.get('Content-Length')
    if content_length:
        try:
            if int(content_length) > 1_048_576:
                return JsonResponse({'detail': 'request_too_large'}, status=413)
        except ValueError:
            return JsonResponse({'detail': 'invalid_content_length'}, status=400)
    expected_token = os.environ.get('FLARE_SYNC_TOKEN', '')
    authorization = request.headers.get('Authorization', '')
    supplied_token = authorization.removeprefix('Bearer ').strip()
    if not expected_token:
        return JsonResponse({'detail': 'sync_not_configured'}, status=503)
    if not authorization.startswith('Bearer ') or not hmac.compare_digest(supplied_token.encode(), expected_token.encode()):
        return JsonResponse({'detail': 'unauthorized'}, status=401)

    items = request.data.get('items') if isinstance(request.data, dict) else None
    if not isinstance(items, list) or not items or len(items) > 100:
        raise ValidationError({'items': 'Provide between 1 and 100 sync items.'})

    accepted = []
    conflicts = []
    with transaction.atomic():
        for item in items:
            if not isinstance(item, dict):
                raise ValidationError({'items': 'Each sync item must be an object.'})
            event_id = item.get('queue_id')
            entity_type = item.get('entity_type')
            payload = item.get('payload')
            raw_created_at = item.get('created_at')
            created_at = parse_datetime(raw_created_at) if isinstance(raw_created_at, str) else None
            if not isinstance(event_id, str) or not 1 <= len(event_id) <= 64:
                raise ValidationError({'queue_id': 'Must be a string of 1-64 characters.'})
            if not isinstance(entity_type, str) or not 1 <= len(entity_type) <= 64:
                raise ValidationError({'entity_type': 'Must be a string of 1-64 characters.'})
            if not isinstance(payload, dict) or created_at is None:
                raise ValidationError({'payload': 'Payload must be an object and created_at must be an ISO timestamp.'})
            if timezone.is_naive(created_at):
                created_at = timezone.make_aware(created_at, timezone.get_current_timezone())

            payload_json = json.dumps(payload, sort_keys=True, separators=(',', ':'))
            entity_id = payload.get('entity_id', '')
            if not isinstance(entity_id, str) or len(entity_id) > 128:
                raise ValidationError({'entity_id': 'Must be a string of at most 128 characters.'})

            existing = SyncEvent.objects.filter(event_id=event_id).first()
            if existing:
                if (existing.entity_type != entity_type or existing.payload_json != payload_json or
                        existing.created_at != created_at):
                    raise EventIDReuse({'event_id': event_id})
            else:
                prior = None
                if entity_id:
                    prior = SyncEvent.objects.filter(
                        entity_type=entity_type,
                        entity_id=entity_id,
                    ).exclude(payload_json=payload_json).order_by('-created_at').first()
                SyncEvent.objects.create(
                    event_id=event_id,
                    entity_type=entity_type,
                    entity_id=entity_id,
                    payload_json=payload_json,
                    created_at=created_at,
                )
                if prior:
                    first_id, second_id = sorted((prior.event_id, event_id))
                    conflict, _ = SyncConflict.objects.get_or_create(
                        first_event_id=first_id,
                        second_event_id=second_id,
                        defaults={'entity_type': entity_type, 'entity_id': entity_id},
                    )
                    conflicts.append({
                        'conflict_id': conflict.id,
                        'entity_type': entity_type,
                        'entity_id': entity_id,
                        'event_ids': [first_id, second_id],
                        'payloads': [prior.payload_json, payload_json] if prior.event_id == first_id else [payload_json, prior.payload_json],
                    })
            accepted.append(event_id)

    return JsonResponse({'accepted': accepted, 'conflicts': conflicts})

@api_view(['GET'])
def sync_conflicts(request):
    expected_token = os.environ.get('FLARE_SYNC_TOKEN', '')
    authorization = request.headers.get('Authorization', '')
    supplied_token = authorization.removeprefix('Bearer ').strip()
    if not expected_token:
        return JsonResponse({'detail': 'sync_not_configured'}, status=503)
    if not authorization.startswith('Bearer ') or not hmac.compare_digest(supplied_token.encode(), expected_token.encode()):
        return JsonResponse({'detail': 'unauthorized'}, status=401)
    conflicts = SyncConflict.objects.order_by('resolved_at', '-created_at')[:100]
    return JsonResponse({
        'conflicts': [{
            'conflict_id': conflict.id,
            'entity_type': conflict.entity_type,
            'entity_id': conflict.entity_id,
            'event_ids': [conflict.first_event_id, conflict.second_event_id],
            'payloads': [
                json.loads(SyncEvent.objects.get(event_id=conflict.first_event_id).payload_json),
                json.loads(SyncEvent.objects.get(event_id=conflict.second_event_id).payload_json),
            ],
            'created_at': conflict.created_at.isoformat(),
            'resolved': conflict.resolved_at is not None,
        } for conflict in conflicts],
    })

@api_view(['POST'])
def sync_resolve(request, conflict_id):
    expected_token = os.environ.get('FLARE_SYNC_TOKEN', '')
    authorization = request.headers.get('Authorization', '')
    supplied_token = authorization.removeprefix('Bearer ').strip()
    if not expected_token:
        return JsonResponse({'detail': 'sync_not_configured'}, status=503)
    if not authorization.startswith('Bearer ') or not hmac.compare_digest(supplied_token.encode(), expected_token.encode()):
        return JsonResponse({'detail': 'unauthorized'}, status=401)

    event_id = request.data.get('event_id') if isinstance(request.data, dict) else None
    with transaction.atomic():
        conflict = SyncConflict.objects.select_for_update().filter(id=conflict_id).first()
        if conflict is None:
            return JsonResponse({'detail': 'conflict_not_found'}, status=404)
        if conflict.resolved_at is not None:
            return JsonResponse({'detail': 'conflict_already_resolved'}, status=409)
        if event_id not in (conflict.first_event_id, conflict.second_event_id):
            raise ValidationError({'event_id': 'Choose one of the preserved conflict versions.'})
        selected = SyncEvent.objects.get(event_id=event_id)
        resolution_id = uuid.uuid4().hex
        SyncEvent.objects.create(
            event_id=resolution_id,
            entity_type=selected.entity_type,
            entity_id=selected.entity_id,
            payload_json=selected.payload_json,
            created_at=timezone.now(),
        )
        conflict.resolved_at = timezone.now()
        conflict.resolution_event_id = resolution_id
        conflict.save(update_fields=['resolved_at', 'resolution_event_id'])
    return JsonResponse({'resolved': True, 'resolution_event_id': resolution_id})
