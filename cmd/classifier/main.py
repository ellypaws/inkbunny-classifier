import gradio as gr
from fastapi import FastAPI, UploadFile, File
from fastapi.responses import JSONResponse
from ultralytics import YOLO
from PIL import Image, ImageFile
import io
import torch
import uvicorn
import os
import requests

from utils import get_latest_weights, decrypt_aes_ctr, save_file

# Automatically get the latest best.pt weights path.
try:
    model_path = get_latest_weights()
    print("Loading model from latest weights:", model_path)
except Exception as e:
    model_path = "yolo11x-cls.pt"
    print(f"Failed to get latest weights: {str(e)}")
    print("Falling back to default model:", model_path)

# Load the model using the latest weights
model = YOLO(model_path)
cuda = torch.cuda
device = 'cuda' if cuda.is_available() else 'cpu'

# Check GPU availability and handle USE_CUDA
use_cuda = os.getenv('USE_CUDA', 'false').lower() == 'true'
if device != 'cuda':
    if use_cuda:
        print("ERROR: CUDA is not available but USE_CUDA is true. Exiting...")
        raise RuntimeError("CUDA is not available. Exiting")
    else:
        print("WARNING: You are using a CPU device. Performance will be significantly slower.")
else:
    print("Using CUDA GPU support.")

model.to(device)


def predict_image(image: Image.Image | str) -> dict[str, float]:
    """Perform prediction on the image and return a dictionary of class probabilities."""
    results = model(source=image, imgsz=224, half=True, device='cuda' if torch.cuda.is_available() else 'cpu')
    probs = results[0].probs.data.cpu().numpy()
    labels = results[0].names
    # Build the result dictionary
    return {labels[i]: float(probs[i]) for i in range(len(labels))}


# FastAPI app
app = FastAPI()

ImageFile.LOAD_TRUNCATED_IMAGES = True
@app.post("/predict")
async def predict(
        file: UploadFile = File(None),
        url: str | None = None,
        key: str | None = None,
        name: str | None = None,
        save: bool = False
):
    data = None
    if url:
        print("Downloading", url)
        response = requests.get(url)
        if response.status_code != 200:
            print("Failed to fetch image from URL:", url)
            return JSONResponse(content={"error": "Could not fetch image from URL"}, status_code=400)
        data = response.content
    elif file:
        data = await file.read()
    else:
        print("No image file or URL provided.")
        return JSONResponse(content={"error": "No image file or URL provided."}, status_code=400)

    # If key is provided, decrypt the input data.
    if key:
        try:
            data = decrypt_aes_ctr(data, key)
        except Exception as e:
            print("Decryption failed:", str(e))
            return JSONResponse(content={"error": f"Decryption failed: {str(e)}"}, status_code=400)

    try:
        image = Image.open(io.BytesIO(data)).convert("RGB")
    except Exception as e:
        print("Failed to convert image:", str(e))
        return JSONResponse(content={"error": f"Failed to process image: {str(e)}"}, status_code=400)

    predictions = predict_image(image)

    # Save the file in predict/{top_class}/{name}.
    if save:
        if not name:
            if file and file.filename:
                name = file.filename
            elif url:
                name = os.path.basename(url)
            else:
                name = "downloaded.jpg"
        saved_path = save_file(data, predictions, name)
        print(f"File saved to: {saved_path}")

    # Include the saved path in the response.
    return JSONResponse(content=predictions)


# Gradio interface
gr_interface = gr.Interface(
    fn=predict_image,
    inputs=gr.Image(type="pil"),
    outputs=gr.Label(num_top_classes=10),
    title="YOLO Classifier"
)

app = gr.mount_gradio_app(app, gr_interface, path="")

# Run server
if __name__ == "__main__":
    port = int(os.getenv('PORT', '7860'))
    uvicorn.run(app, host="0.0.0.0", port=port)
