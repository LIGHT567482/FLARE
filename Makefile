.PHONY: build up down flared bridge django frontend all clean

build:
	cd daemon/go-flared && go build -o /tmp/flared ./cmd/flared
	cd native/cpp-crypto && make clean && make
	cd frontend/flare-ui && npm i --no-audit --no-fund

flared:
	/tmp/flared

bridge:
	cd bridge/python-bridge && ./gen_certs.sh 2>/dev/null || true && PYTHONPATH=$(PWD)/bridge/python-bridge python3 run.py

django:
	cd backend/django-flare && python3 manage.py migrate --noinput && python3 manage.py runserver 127.0.0.1:8000

frontend:
	cd frontend/flare-ui && npx vite --host 127.0.0.1 --port 5173

all: build
	./run_local.sh start

down:
	./run_local.sh stop

clean:
	rm -f /tmp/flared /tmp/flared.pid /tmp/flare-*.pid
	cd native/cpp-crypto && make clean
