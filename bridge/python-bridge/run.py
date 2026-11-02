#!/usr/bin/env python3
import os
import sys
import subprocess

BASE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(BASE, "app"))

if __name__ == "__main__":
    env = os.environ.copy()
    cmd = [
        sys.executable, "-m", "uvicorn",
        "app.main:app",
        "--host", env.get("BRIDGE_HOST", "127.0.0.1"),
        "--port", env.get("BRIDGE_PORT", "8443"),
        "--workers", env.get("BRIDGE_WORKERS", "1"),
        "--timeout-keep-alive", "60",
    ]
    if env.get("TLS_ENABLED", "true").lower() == "true":
        cert = env.get("TLS_CERT", os.path.join(BASE, "cert.pem"))
        key = env.get("TLS_KEY", os.path.join(BASE, "key.pem"))
        if os.path.exists(cert) and os.path.exists(key):
            cmd.extend(["--ssl-certfile", cert, "--ssl-keyfile", key])
    subprocess.run(cmd, env=env)
