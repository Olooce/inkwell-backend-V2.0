import sys
import json
import requests

# Ensure access token and prompt are provided
if len(sys.argv) < 3:
    print(json.dumps({"status": "error", "message": "Usage: script.py <access_token> <prompt>"}))
    sys.exit(1)

access_token = sys.argv[1]
prompt = sys.argv[2]

# Hugging Face Inference API URL
api_url = "https://api-inference.huggingface.co/models/DeepFloyd/IF-I-M-v1.0"
headers = {"Authorization": f"Bearer {access_token}"}

def generate_image(prompt):
    payload = {"inputs": prompt}

    try:
        response = requests.post(api_url, headers=headers, json=payload)

        # Check content type to determine if it's an image or an error message
        if response.headers.get("content-type", "").startswith("image"):
            path = "generated_image.png"
            with open(path, "wb") as f:
                f.write(response.content)
            return json.dumps({"status": "success", "path": path})

        # If not an image, it's likely an error message
        else:
            return json.dumps({"status": "error", "message": response.text})

    except Exception as e:
        return json.dumps({"status": "error", "message": str(e)})

if __name__ == "__main__":
    print(generate_image(prompt))
