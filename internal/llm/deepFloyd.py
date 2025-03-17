import sys
import json
import torch
from diffusers import DiffusionPipeline

# Initialize DeepFloyd IF
device = "cuda" if torch.cuda.is_available() else "cpu"
pipe = DiffusionPipeline.from_pretrained("DeepFloyd/IF-I-M-v1.0").to(device)

def generate_image(prompt):
    try:
        image = pipe(prompt).images[0]
        path = "generated_image.png"
        image.save(path)
        return json.dumps({"status": "success", "path": path})
    except Exception as e:
        return json.dumps({"status": "error", "message": str(e)})

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print(json.dumps({"status": "error", "message": "No prompt provided"}))
        sys.exit(1)

    prompt = sys.argv[1]
    print(generate_image(prompt))
