import uuid
from django.db import models

def new_id():
    return uuid.uuid4().hex

class SystemNodeConfig(models.Model):
    node_id = models.CharField(primary_key=True, max_length=64)
    hardware_hash = models.CharField(max_length=128, unique=True)
    license_key = models.CharField(max_length=256)
    activation_status = models.IntegerField(default=0)
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)

    class Meta:
        db_table = 'system_node_config'

class LocalAuditLog(models.Model):
    id = models.CharField(primary_key=True, max_length=64, default=new_id)
    event_type = models.CharField(max_length=64)
    payload_json = models.TextField()
    hash_chain = models.CharField(max_length=128)
    created_at = models.DateTimeField(auto_now_add=True)

    class Meta:
        db_table = 'local_audit_logs'

class OfflineSyncQueue(models.Model):
    class Status(models.IntegerChoices):
        PENDING = 0
        PROCESSING = 1
        COMPLETED = 2
        FAILED = 3

    queue_id = models.CharField(primary_key=True, max_length=64, default=new_id)
    entity_type = models.CharField(max_length=64)
    payload_json = models.TextField()
    status = models.IntegerField(choices=Status.choices, default=Status.PENDING)
    attempts = models.IntegerField(default=0)
    last_attempt_at = models.DateTimeField(null=True, blank=True)
    created_at = models.DateTimeField(auto_now_add=True)

    class Meta:
        db_table = 'offline_sync_queue'
        ordering = ['created_at']
