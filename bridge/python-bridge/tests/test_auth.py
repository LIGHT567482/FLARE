import unittest
from unittest.mock import AsyncMock, patch

from fastapi.testclient import TestClient

from app import main


class BridgeAuthTests(unittest.TestCase):
    def setUp(self):
        self.token_patch = patch.object(main, 'BRIDGE_AUTH_TOKEN', 'test-bridge-token')
        self.token_patch.start()
        self.addCleanup(self.token_patch.stop)
        self.client = TestClient(main.app)

    def test_health_is_public_but_ipc_requires_authentication(self):
        self.assertEqual(self.client.get('/health').status_code, 200)
        response = self.client.post('/flare-ipc', json={'cmd': 'status'})
        self.assertEqual(response.status_code, 401)

    def test_ipc_rejects_invalid_token(self):
        response = self.client.post(
            '/ipc',
            json={'cmd': 'status'},
            headers={'Authorization': 'Bearer wrong-token'},
        )
        self.assertEqual(response.status_code, 401)

    def test_authenticated_ipc_request_is_forwarded(self):
        with patch.object(main, 'call_flared', new=AsyncMock(return_value={'engine': 'ok'})):
            response = self.client.post(
                '/flare-ipc',
                json={'cmd': 'status'},
                headers={'Authorization': 'Bearer test-bridge-token'},
            )
        self.assertEqual(response.status_code, 200)
        self.assertEqual(response.json(), {'engine': 'ok'})
