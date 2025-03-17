import sys
import json
import torch
from diffusers import DiffusionPipeline

# Ensure access token is provided
if len(sys.argv) < 3:
    print(json.dumps({"status": "error", "message": "Usage: script.py <access_token> <prompt>"}))
    sys.exit(1)

# Read access token and prompt from command line arguments
access_token = sys.argv[1]
prompt = sys.argv[2]

# Initialize DeepFloyd IF
device = "cuda" if torch.cuda.is_available() else "cpu"
pipe = DiffusionPipeline.from_pretrained("DeepFloyd/IF-I-M-v1.0", token=access_token).to(device)

def generate_image(prompt):
    try:
        image = pipe(prompt).images[0]
        path = "generated_image.png"
        image.save(path)
        return json.dumps({"status": "success", "path": path})
    except Exception as e:
        return json.dumps({"status": "error", "message": str(e)})

if __name__ == "__main__":
    print(generate_image(prompt))
