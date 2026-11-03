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

class SyncEvent(models.Model):
    event_id = models.CharField(primary_key=True, max_length=64)
    entity_type = models.CharField(max_length=64)
    entity_id = models.CharField(max_length=128, blank=True)
    payload_json = models.TextField()
    created_at = models.DateTimeField()
    received_at = models.DateTimeField(auto_now_add=True)

    class Meta:
        db_table = 'sync_events'
        ordering = ['created_at']
        indexes = [
            models.Index(fields=['entity_type', 'entity_id']),
        ]

class SyncConflict(models.Model):
    id = models.CharField(primary_key=True, max_length=64, default=new_id)
    entity_type = models.CharField(max_length=64)
    entity_id = models.CharField(max_length=128)
    first_event_id = models.CharField(max_length=64)
    second_event_id = models.CharField(max_length=64)
    created_at = models.DateTimeField(auto_now_add=True)
    resolved_at = models.DateTimeField(null=True, blank=True)
    resolution_event_id = models.CharField(max_length=64, blank=True)

    class Meta:
        db_table = 'sync_conflicts'
        constraints = [
            models.UniqueConstraint(
                fields=['first_event_id', 'second_event_id'],
                name='unique_sync_conflict_pair',
            ),
        ]
        ordering = ['created_at']
