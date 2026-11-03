from django.contrib import admin
from .models import SystemNodeConfig, LocalAuditLog, OfflineSyncQueue, SyncEvent, SyncConflict

admin.site.register(SystemNodeConfig)
admin.site.register(LocalAuditLog)
admin.site.register(OfflineSyncQueue)
admin.site.register(SyncEvent)
admin.site.register(SyncConflict)
