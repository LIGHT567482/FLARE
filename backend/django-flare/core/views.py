import json
from django.http import JsonResponse
from rest_framework.decorators import api_view
from .models import OfflineSyncQueue

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
