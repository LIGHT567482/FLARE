import asyncio
import json
import os
import socket
import ssl
from contextlib import asynccontextmanager
from typing import Any, Dict, Optional

from fastapi import FastAPI, HTTPException, Request
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel

FLARED_SOCKET = os.getenv("FLARED_SOCKET", "/tmp/flared.sock")
BRIDGE_HOST = os.getenv("BRIDGE_HOST", "127.0.0.1")
BRIDGE_PORT = int(os.getenv("BRIDGE_PORT", "8443"))
TLS_CERT = os.getenv("TLS_CERT", "cert.pem")
TLS_KEY = os.getenv("TLS_KEY", "key.pem")
TLS_ENABLED = os.getenv("TLS_ENABLED", "true").lower() == "true"
MAX_CONCURRENT = int(os.getenv("MAX_CONCURRENT", "100"))
READ_TIMEOUT = float(os.getenv("READ_TIMEOUT", "5.0"))
CONNECT_TIMEOUT = float(os.getenv("CONNECT_TIMEOUT", "2.0"))
MAX_LINE_BYTES = int(os.getenv("MAX_LINE_BYTES", "1048576"))  # 1MB

sem = asyncio.Semaphore(MAX_CONCURRENT)

class IpcCmd(BaseModel):
    cmd: str
    payload: Optional[Dict[str, Any]] = None
    entity_type: Optional[str] = None
    limit: Optional[int] = None

@asynccontextmanager
async def lifespan(app: FastAPI):
    # health check socket exists
    app.state.flared_socket = FLARED_SOCKET
    yield

app = FastAPI(title="FLARE IPC Bridge", version="1.0.0", lifespan=lifespan)
app.add_middleware(
    CORSMiddleware,
    allow_origins=["http://localhost:5173", "http://127.0.0.1:5173"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

async def call_flared(cmd: IpcCmd) -> Dict[str, Any]:
    async with sem:
        loop = asyncio.get_running_loop()
        fut = loop.run_in_executor(None, _sync_call, cmd.dict())
        try:
            return await asyncio.wait_for(fut, timeout=READ_TIMEOUT + CONNECT_TIMEOUT + 2)
        except asyncio.TimeoutError:
            raise HTTPException(status_code=504, detail="bridge_timeout")

def _sync_call(payload: Dict[str, Any]) -> Dict[str, Any]:
    req = {"cmd": payload.get("cmd")}
    if payload.get("payload") is not None:
        req["payload"] = payload["payload"]
    if payload.get("entity_type") is not None:
        req["entity_type"] = payload["entity_type"]
    if payload.get("limit") is not None:
        req["limit"] = payload["limit"]
    line = json.dumps(req, separators=(",", ":")) + "\n"
    if len(line.encode()) > MAX_LINE_BYTES:
        raise ValueError("request_too_large")

    try:
        s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        s.settimeout(CONNECT_TIMEOUT)
        s.connect(FLARED_SOCKET)
        s.settimeout(READ_TIMEOUT)
        s.sendall(line.encode())
        data = s.recv(1024 * 1024).rstrip(b"\n\r")
        s.close()
        if not data:
            return {"error": "empty_response"}
        try:
            return json.loads(data.decode())
        except Exception:
            return {"raw": data.decode(errors="replace")}
    except OSError as e:
        raise OSError(str(e))

@app.post("/ipc")
async def ipc_endpoint(cmd: IpcCmd):
    try:
        return await call_flared(cmd)
    except OSError as e:
        raise HTTPException(status_code=503, detail=f"flared_unreachable:{e}")
    except ValueError as e:
        raise HTTPException(status_code=413, detail=str(e))

@app.get("/health")
async def health():
    return {"status": "ok", "flared_socket": FLARED_SOCKET, "tls": TLS_ENABLED}

@app.post("/flare-ipc")
async def flare_ipc(req: Request):
    body = await req.body()
    try:
        payload = json.loads(body.decode())
    except Exception:
        raise HTTPException(status_code=400, detail="bad_json")
    try:
        return await call_flared(IpcCmd(**payload))
    except TypeError:
        # allow raw shape
        return await call_flared(IpcCmd(cmd=payload.get("cmd"), **{k: v for k, v in payload.items() if k != "cmd"}))

if __name__ == "__main__":
    import uvicorn
    ssl_ctx = None
    if TLS_ENABLED and os.path.exists(TLS_CERT) and os.path.exists(TLS_KEY):
        ssl_ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        ssl_ctx.load_cert_chain(TLS_CERT, TLS_KEY)
    uvicorn.run("app.main:app", host=BRIDGE_HOST, port=BRIDGE_PORT, ssl_context=ssl_ctx, reload=False)
