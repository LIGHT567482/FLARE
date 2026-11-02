## Build instructions

### Python/Django
cd backend/django-flare && pip install -r requirements.txt && python manage.py migrate && python manage.py runserver

### Go daemon
cd daemon/go-flared && go build -o /tmp/flared ./cmd/flared && /tmp/flared

### C++ crypto
cd native/cpp-crypto && make && make install (optional)

### Frontend
cd frontend/flare-ui && npm i && npm run dev
