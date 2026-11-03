import core.models
from django.db import migrations, models


class Migration(migrations.Migration):

    dependencies = [
        ('core', '0001_initial'),
    ]

    operations = [
        migrations.CreateModel(
            name='SyncEvent',
            fields=[
                ('event_id', models.CharField(max_length=64, primary_key=True, serialize=False)),
                ('entity_type', models.CharField(max_length=64)),
                ('entity_id', models.CharField(blank=True, max_length=128)),
                ('payload_json', models.TextField()),
                ('created_at', models.DateTimeField()),
                ('received_at', models.DateTimeField(auto_now_add=True)),
            ],
            options={
                'db_table': 'sync_events',
                'ordering': ['created_at'],
            },
        ),
        migrations.CreateModel(
            name='SyncConflict',
            fields=[
                ('id', models.CharField(default=core.models.new_id, max_length=64, primary_key=True, serialize=False)),
                ('entity_type', models.CharField(max_length=64)),
                ('entity_id', models.CharField(max_length=128)),
                ('first_event_id', models.CharField(max_length=64)),
                ('second_event_id', models.CharField(max_length=64)),
                ('created_at', models.DateTimeField(auto_now_add=True)),
            ],
            options={
                'db_table': 'sync_conflicts',
                'ordering': ['created_at'],
            },
        ),
        migrations.AddIndex(
            model_name='syncevent',
            index=models.Index(fields=['entity_type', 'entity_id'], name='sync_events_entity__1a9c0a_idx'),
        ),
        migrations.AddConstraint(
            model_name='syncconflict',
            constraint=models.UniqueConstraint(fields=('first_event_id', 'second_event_id'), name='unique_sync_conflict_pair'),
        ),
    ]
