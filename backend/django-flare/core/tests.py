import json
import os
from unittest.mock import patch

from django.test import TestCase

from .models import SyncConflict, SyncEvent


class SyncPushTests(TestCase):
    def setUp(self):
        self.token_patch = patch.dict(os.environ, {'FLARE_SYNC_TOKEN': 'test-token'})
        self.token_patch.start()
        self.addCleanup(self.token_patch.stop)

    def push(self, items, token='test-token'):
        return self.client.post(
            '/api/sync/push/',
            data=json.dumps({'items': items}),
            content_type='application/json',
            HTTP_AUTHORIZATION=f'Bearer {token}',
        )

    def item(self, event_id, payload, timestamp='2026-11-02T12:00:00Z'):
        return {
            'queue_id': event_id,
            'entity_type': 'note',
            'payload': payload,
            'created_at': timestamp,
        }

    def test_requires_configured_bearer_token(self):
        with patch.dict(os.environ, {'FLARE_SYNC_TOKEN': ''}):
            response = self.push([self.item('event-1', {'entity_id': 'note-1'})])
        self.assertEqual(response.status_code, 503)

        response = self.push([self.item('event-1', {'entity_id': 'note-1'})], token='wrong')
        self.assertEqual(response.status_code, 401)

    def test_is_idempotent_and_preserves_conflicting_versions(self):
        first = self.item('event-1', {'entity_id': 'note-1', 'text': 'first'})
        response = self.push([first])
        self.assertEqual(response.status_code, 200, response.content)
        self.assertEqual(response.json()['accepted'], ['event-1'])

        response = self.push([first])
        self.assertEqual(response.status_code, 200, response.content)
        self.assertEqual(SyncEvent.objects.count(), 1)

        second = self.item('event-2', {'entity_id': 'note-1', 'text': 'second'})
        response = self.push([second])
        self.assertEqual(response.status_code, 200, response.content)
        self.assertEqual(SyncEvent.objects.count(), 2)
        self.assertEqual(SyncConflict.objects.count(), 1)
        self.assertEqual(response.json()['conflicts'][0]['entity_id'], 'note-1')

    def test_rejects_reuse_of_event_id_with_different_content(self):
        self.push([self.item('event-1', {'entity_id': 'note-1', 'text': 'first'})])
        response = self.push([self.item('event-1', {'entity_id': 'note-1', 'text': 'changed'})])
        self.assertEqual(response.status_code, 409)
        self.assertEqual(SyncEvent.objects.count(), 1)

    def test_conflict_can_be_resolved_by_selecting_a_preserved_version(self):
        self.push([self.item('event-1', {'entity_id': 'note-1', 'text': 'first'})])
        response = self.push([self.item('event-2', {'entity_id': 'note-1', 'text': 'second'})])
        conflict_id = response.json()['conflicts'][0]['conflict_id']

        response = self.client.post(
            f'/api/sync/conflicts/{conflict_id}/resolve/',
            data=json.dumps({'event_id': 'event-2'}),
            content_type='application/json',
            HTTP_AUTHORIZATION='Bearer test-token',
        )
        self.assertEqual(response.status_code, 200, response.content)
        self.assertTrue(response.json()['resolved'])
        self.assertIsNotNone(SyncConflict.objects.get(id=conflict_id).resolved_at)

        response = self.client.get('/api/sync/conflicts/', HTTP_AUTHORIZATION='Bearer test-token')
        self.assertEqual(response.status_code, 200)
        self.assertTrue(response.json()['conflicts'][0]['resolved'])
