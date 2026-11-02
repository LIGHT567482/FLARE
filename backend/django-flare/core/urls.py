from django.urls import path
from . import views

urlpatterns = [
    path('health/', views.health),
    path('sync/queue/', views.queue_item),
    path('sync/pull/', views.pull_queue),
]
