import json
import os
import tempfile
import zipfile
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path

from ultralytics import YOLO

BASE_DIR = Path(__file__).resolve().parent.parent
MODEL_PATH = Path(
    os.getenv("YOLO_MODEL_PATH", BASE_DIR / "yolo26s-cls.pt" / "best")
)
UPLOADS_DIR = Path(os.getenv("UPLOADS_DIR", BASE_DIR.parent / "frontend" / "public" / "uploads"))
PORT = int(os.getenv("YOLO_PORT", "8086"))


def prepare_model_path(path: Path) -> Path:
    if not path.is_dir():
        return path

    archive = Path(tempfile.mkstemp(prefix="resqio-yolo-", suffix=".pt")[1])
    with zipfile.ZipFile(
        archive, "w", compression=zipfile.ZIP_DEFLATED, strict_timestamps=False
    ) as output:
        for file_path in path.rglob("*"):
            if file_path.is_file():
                output.write(file_path, file_path.relative_to(path.parent))
    return archive


MODEL_FILE_PATH = prepare_model_path(MODEL_PATH)
model = YOLO(str(MODEL_FILE_PATH), task="classify")


def resolve_image(image_url: str) -> Path:
    image_path = Path(image_url)
    if image_url.startswith("/uploads/"):
        image_path = UPLOADS_DIR / image_url.removeprefix("/uploads/")
    if not image_path.is_file():
        raise FileNotFoundError("uploaded issue image was not found")
    return image_path


class Handler(BaseHTTPRequestHandler):
    def send_json(self, status: int, payload: dict):
        body = json.dumps(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        if self.path == "/health":
            self.send_json(200, {"status": "ok", "model": str(MODEL_PATH)})
            return
        self.send_json(404, {"error": "not found"})

    def do_POST(self):
        if self.path != "/predict":
            self.send_json(404, {"error": "not found"})
            return
        try:
            length = int(self.headers.get("Content-Length", 0))
            payload = json.loads(self.rfile.read(length))
            image_path = resolve_image(payload["image_url"])
            result = model.predict(source=str(image_path), verbose=False)[0]
            top_class = int(result.probs.top1)
            self.send_json(200, {
                "predicted_class": result.names[top_class],
                "confidence_score": float(result.probs.top1conf),
            })
        except Exception as error:
            self.send_json(400, {"error": str(error)})


if __name__ == "__main__":
    HTTPServer(("0.0.0.0", PORT), Handler).serve_forever()