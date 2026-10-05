#!/usr/bin/env bash
# 本地向量服务（OpenAI 兼容 /v1/embeddings），给 Phase 9 的向量检索用。
#
# 为什么自建：任务书/修订清单要求向量检索，但 DeepSeek 没有 /embeddings 端点（实测 HTTP 404），
# 也不想为了它再引入外部 Key —— 本机已有 Python + sentence-transformers + torch，直接本地跑，
# 零外部依赖、零成本、语料不出机器。
#
# 用法：bash scripts/dev-embed-server.sh          # 前台运行（Ctrl-C 停）
#       bash scripts/dev-embed-server.sh -d       # 后台运行，日志 .run/embed.log
# 端口：EMBED_PORT（默认 18901）  模型：EMBED_MODEL（默认 BAAI/bge-small-zh-v1.5）
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PORT="${EMBED_PORT:-18901}"
MODEL="${EMBED_MODEL:-BAAI/bge-small-zh-v1.5}"
# HuggingFace 国内镜像（官方源在部分网络下很慢）
export HF_ENDPOINT="${HF_ENDPOINT:-https://hf-mirror.com}"

SERVER_PY="${REPO_ROOT}/.run/embed_server.py"
mkdir -p "${REPO_ROOT}/.run"

cat > "${SERVER_PY}" <<'PY'
"""极简 OpenAI 兼容 embedding 服务（只用标准库 + sentence-transformers）。"""
import json, os, sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

from sentence_transformers import SentenceTransformer

MODEL_NAME = os.environ.get("EMBED_MODEL", "BAAI/bge-small-zh-v1.5")
PORT = int(os.environ.get("EMBED_PORT", "18901"))
print(f"加载模型 {MODEL_NAME} …", flush=True)
model = SentenceTransformer(MODEL_NAME)
print(f"模型就绪，监听 127.0.0.1:{PORT}", flush=True)


class Handler(BaseHTTPRequestHandler):
    def _send(self, code, payload):
        raw = json.dumps(payload, ensure_ascii=False).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def do_POST(self):
        if not self.path.rstrip("/").endswith("/embeddings"):
            return self._send(404, {"error": {"message": "只实现了 /v1/embeddings"}})
        length = int(self.headers.get("Content-Length") or 0)
        try:
            body = json.loads(self.rfile.read(length) or b"{}")
        except Exception as err:  # noqa: BLE001
            return self._send(400, {"error": {"message": f"请求体不是合法 JSON: {err}"}})
        inputs = body.get("input")
        if isinstance(inputs, str):
            inputs = [inputs]
        if not isinstance(inputs, list) or not inputs:
            return self._send(400, {"error": {"message": "input 必须是非空字符串或字符串数组"}})
        vectors = model.encode(inputs, normalize_embeddings=True, batch_size=32)
        data = [
            {"object": "embedding", "index": i, "embedding": [float(x) for x in vec]}
            for i, vec in enumerate(vectors)
        ]
        self._send(200, {
            "object": "list",
            "model": MODEL_NAME,
            "data": data,
            "usage": {"prompt_tokens": 0, "total_tokens": 0},
        })

    def log_message(self, *args):
        pass


if __name__ == "__main__":
    ThreadingHTTPServer(("127.0.0.1", PORT), Handler).serve_forever()
PY

if [[ "${1:-}" == "-d" ]]; then
  ( nohup python3 "${SERVER_PY}" > "${REPO_ROOT}/.run/embed.log" 2>&1 & echo $! > "${REPO_ROOT}/.run/embed.pid" )
  echo "已后台启动，PID $(cat "${REPO_ROOT}/.run/embed.pid")，日志 ${REPO_ROOT}/.run/embed.log"
  echo "就绪检查：curl -s http://127.0.0.1:${PORT}/v1/embeddings -H 'Content-Type: application/json' -d '{\"input\":\"测试\"}' | head -c 120"
else
  EMBED_MODEL="${MODEL}" EMBED_PORT="${PORT}" exec python3 "${SERVER_PY}"
fi
