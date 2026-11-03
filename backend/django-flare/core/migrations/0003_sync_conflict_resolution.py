from django.db import migrations, models


class Migration(migrations.Migration):

    dependencies = [
        ('core', '0002_sync_event_conflict'),
    ]

    operations = [
        migrations.AddField(
            model_name='syncconflict',
            name='resolved_at',
            field=models.DateTimeField(blank=True, null=True),
        ),
        migrations.AddField(
            model_name='syncconflict',
            name='resolution_event_id',
            field=models.CharField(blank=True, max_length=64),
        ),
    ]
