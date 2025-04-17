from multiprocessing.dummy import freeze_support
import os

from ultralytics import YOLO
import torch

if __name__ == '__main__':
    freeze_support()
    # Load a pretrained YOLOv11 classification model
    model = YOLO('yolo11x-cls.pt')

    # Check GPU availability and handle USE_CUDA
    use_cuda = os.getenv('USE_CUDA', 'false').lower() == 'true'
    if torch.cuda.is_available():
        print("CUDA is available. Using GPU.")
        model.to('cuda')
    else:
        if use_cuda:
            print("ERROR: CUDA is not available but USE_CUDA is true. Exiting...")
            exit(1)
        else:
            print("WARNING: CUDA is not available. Using CPU (performance will be significantly slower).")

    # Train the model on your dataset
    results = model.train(data='./dataset', epochs=300, imgsz=640, device='cuda' if torch.cuda.is_available() else 'cpu', profile=True)
