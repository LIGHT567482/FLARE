from django.urls import path
from . import views

urlpatterns = [
    path('health/', views.health),
    path('sync/queue/', views.queue_item),
    path('sync/pull/', views.pull_queue),
    path('sync/push/', views.sync_push),
    path('sync/conflicts/', views.sync_conflicts),
    path('sync/conflicts/<str:conflict_id>/resolve/', views.sync_resolve),
]
