import json
from http.server import BaseHTTPRequestHandler, HTTPServer


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/v1/models":
            self.send_json({"object": "list", "data": [{"id": "gpt-5-codex", "object": "model", "owned_by": "ci"}]})
        else:
            self.send_error(404)

    def do_POST(self):
        length = int(self.headers.get("Content-Length", "0"))
        body = json.loads(self.rfile.read(length) or "{}")
        if self.path == "/v1/responses":
            self.send_json({"id": "resp_ci", "object": "response", "status": "completed", "model": body.get("model"), "output": [{"type": "message", "role": "assistant", "content": [{"type": "output_text", "text": "fake upstream ok"}]}]})
        else:
            self.send_error(404)

    def send_json(self, payload):
        raw = json.dumps(payload).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def log_message(self, *_):
        pass


HTTPServer(("0.0.0.0", 18080), Handler).serve_forever()
