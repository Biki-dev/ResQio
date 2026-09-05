import os
import sys

# Auto re-exec with virtualenv python if run with older or system Python (< 3.10)
current_dir = os.path.dirname(os.path.abspath(__file__))
venv_python = os.path.join(current_dir, ".venv", "bin", "python")
if sys.version_info < (3, 10) and os.path.isfile(venv_python):
    os.execv(venv_python, [venv_python] + sys.argv)

import json
import logging
from http.server import HTTPServer, BaseHTTPRequestHandler

# Native zero-dependency .env loader (eliminates 'missing dotenv' error)
def load_env_file(filepath: str):
    if os.path.isfile(filepath):
        try:
            with open(filepath, "r", encoding="utf-8") as f:
                for line in f:
                    line = line.strip()
                    if line and not line.startswith("#") and "=" in line:
                        k, v = line.split("=", 1)
                        k = k.strip()
                        v = v.strip().strip("\"'")
                        if k and k not in os.environ:
                            os.environ[k] = v
        except Exception:
            pass

# Load environment variables from backend/.env or root .env
load_env_file(os.path.join(current_dir, "..", ".env"))
load_env_file(os.path.join(current_dir, "..", "..", ".env"))
load_env_file(".env")

from sentence_transformers import SentenceTransformer, util

logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s")

# Initialize Model
MODEL_NAME = os.environ.get("EMBEDDING_MODEL", "sentence-transformers/all-MiniLM-L6-v2")
logging.info(f"Loading SentenceTransformer model: {MODEL_NAME}...")
model = SentenceTransformer(MODEL_NAME)
logging.info("Model loaded successfully.")

# Initialize Pinecone if API key is present
PINECONE_API_KEY = os.environ.get("PINECONE_API_KEY", "").strip()
PINECONE_INDEX_NAME = os.environ.get("PINECONE_INDEX_NAME", "disaster-relief-index")
pinecone_index = None

if PINECONE_API_KEY:
    try:
        from pinecone import Pinecone
        pc = Pinecone(api_key=PINECONE_API_KEY)
        existing_indexes = [idx.name for idx in pc.list_indexes()]
        if PINECONE_INDEX_NAME not in existing_indexes:
            logging.info(f"Creating Pinecone index '{PINECONE_INDEX_NAME}' with dimension 384...")
            from pinecone import ServerlessSpec
            pc.create_index(
                name=PINECONE_INDEX_NAME,
                dimension=384,
                metric="cosine",
                spec=ServerlessSpec(cloud="aws", region="us-east-1")
            )
        pinecone_index = pc.Index(PINECONE_INDEX_NAME)
        logging.info(f"Pinecone initialized with index '{PINECONE_INDEX_NAME}'")
    except Exception as e:
        logging.warning(f"Failed to initialize Pinecone: {e}. Falling back to local vector similarity.")
        pinecone_index = None
else:
    logging.info("No PINECONE_API_KEY found. Operating in local cosine similarity mode.")

VERNACULAR_RELIEF_MAP = {
    "pani": "water drinking water",
    "paani": "water drinking water",
    "jal": "water drinking water",
    "khana": "food meals rations",
    "roti": "food bread rations",
    "chawal": "rice food rations",
    "doodh": "milk baby infant food",
    "dawa": "medicine medical first aid",
    "dawai": "medicine medical first aid",
    "kapda": "clothes clothing blanket",
    "chhat": "shelter tent tarp",
    "tambu": "tent shelter tarp",
}

def enrich_text(text: str) -> str:
    words = text.lower().split()
    additions = []
    for w in words:
        if w in VERNACULAR_RELIEF_MAP:
            additions.append(VERNACULAR_RELIEF_MAP[w])
    if additions:
        return f"{text} {' '.join(additions)}"
    return text

local_inventory_vectors = {}

class EmbeddingRequestHandler(BaseHTTPRequestHandler):
    def _send_json(self, status_code, data):
        response = json.dumps(data).encode("utf-8")
        self.send_response(status_code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(response)))
        self.end_headers()
        self.wfile.write(response)

    def do_GET(self):
        if self.path == "/health" or self.path == "/":
            self._send_json(200, {
                "status": "ok",
                "model": MODEL_NAME,
                "dimension": 384,
                "pinecone_enabled": pinecone_index is not None,
                "index_name": PINECONE_INDEX_NAME if pinecone_index is not None else None
            })
            return
        self._send_json(404, {"error": "Not found"})

    def do_POST(self):
        content_length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(content_length)
        try:
            payload = json.loads(body.decode("utf-8")) if body else {}
        except Exception:
            self._send_json(400, {"error": "Invalid JSON body"})
            return

        if self.path == "/embed":
            texts = []
            if "text" in payload:
                texts = [enrich_text(payload["text"])]
            elif "texts" in payload:
                texts = [enrich_text(t) for t in payload["texts"]]
            else:
                self._send_json(400, {"error": "Expected 'text' or 'texts' in payload"})
                return

            embeddings = model.encode(texts).tolist()
            self._send_json(200, {
                "embeddings": embeddings,
                "dimension": len(embeddings[0]) if embeddings else 384
            })
            return

        elif self.path == "/upsert":
            items = payload.get("items", [])
            if not items:
                self._send_json(400, {"error": "Expected non-empty 'items' array"})
                return

            texts = [enrich_text(item.get("text", "")) for item in items]
            embeddings = model.encode(texts).tolist()

            vectors_to_upsert = []
            results = {}
            for i, item in enumerate(items):
                item_id = item["id"]
                emb = embeddings[i]
                metadata = item.get("metadata", {})
                metadata["text"] = item.get("text", "")
                if "category" in item:
                    metadata["category"] = item["category"]

                results[item_id] = emb
                local_inventory_vectors[item_id] = {
                    "vector": emb,
                    "text": item.get("text", ""),
                    "category": item.get("category", "")
                }
                vectors_to_upsert.append({
                    "id": item_id,
                    "values": emb,
                    "metadata": metadata
                })

            if pinecone_index:
                try:
                    pinecone_index.upsert(vectors=vectors_to_upsert)
                except Exception as e:
                    logging.error(f"Pinecone upsert error: {e}")

            self._send_json(200, {
                "upserted": len(vectors_to_upsert),
                "pinecone_used": pinecone_index is not None,
                "embeddings": results
            })
            return

        elif self.path == "/match":
            query_str = payload.get("query", "").strip()
            if not query_str:
                self._send_json(400, {"error": "Query string is required"})
                return

            top_k = int(payload.get("top_k", 5))
            enriched_query = enrich_text(query_str)
            query_vector = model.encode(enriched_query)
            query_vector_list = query_vector.tolist()

            candidates = payload.get("candidates", [])
            matches = []

            if candidates:
                for cand in candidates:
                    cand_id = cand.get("id", "")
                    cand_text = cand.get("text", "")
                    cand_emb = None
                    if "vector" in cand and cand["vector"]:
                        cand_emb = cand["vector"]
                    elif cand_id in local_inventory_vectors:
                        cand_emb = local_inventory_vectors[cand_id]["vector"]
                    elif cand_text:
                        cand_emb = model.encode(enrich_text(cand_text)).tolist()

                    score = 0.0
                    if cand_emb:
                        score = float(util.cos_sim(query_vector, cand_emb).item())

                    matches.append({
                        "id": cand_id,
                        "text": cand_text,
                        "category": cand.get("category", ""),
                        "score": round(score, 4)
                    })
                matches.sort(key=lambda x: x["score"], reverse=True)
                matches = matches[:top_k]
            elif pinecone_index:
                try:
                    response = pinecone_index.query(
                        vector=query_vector_list,
                        top_k=top_k,
                        include_metadata=True
                    )
                    for m in response.get("matches", []):
                        matches.append({
                            "id": m["id"],
                            "score": round(m["score"], 4),
                            "metadata": m.get("metadata", {}),
                            "text": m.get("metadata", {}).get("text", "")
                        })
                except Exception as e:
                    logging.error(f"Pinecone query error: {e}")
            else:
                for cand_id, data in local_inventory_vectors.items():
                    score = float(util.cos_sim(query_vector, data["vector"]).item())
                    matches.append({
                        "id": cand_id,
                        "text": data["text"],
                        "category": data["category"],
                        "score": round(score, 4)
                    })
                matches.sort(key=lambda x: x["score"], reverse=True)
                matches = matches[:top_k]

            self._send_json(200, {
                "query": query_str,
                "matches": matches,
                "query_vector": query_vector_list
            })
            return

        self._send_json(404, {"error": "Not found"})

def run_server(port=8085):
    server_address = ("", port)
    httpd = HTTPServer(server_address, EmbeddingRequestHandler)
    logging.info(f"Embedding microservice running on http://localhost:{port}")
    try:
        httpd.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        httpd.server_close()
        logging.info("Embedding microservice stopped.")

if __name__ == "__main__":
    port = int(os.environ.get("EMBEDDING_PORT", 8085))
    run_server(port)
