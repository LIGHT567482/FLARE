from django.contrib import admin
from .models import SystemNodeConfig, LocalAuditLog, OfflineSyncQueue

admin.site.register(SystemNodeConfig)
admin.site.register(LocalAuditLog)
admin.site.register(OfflineSyncQueue)
