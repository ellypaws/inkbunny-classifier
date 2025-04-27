FROM python:3.12-slim

WORKDIR /app

# Install system dependencies
RUN apt-get update && apt-get install -y \
    python3-pip \
    && rm -rf /var/lib/apt/lists/*

# Copy requirements first to leverage Docker cache
COPY requirements.txt .
RUN pip3 install --no-cache-dir -r requirements.txt
RUN pip3 install --no-cache-dir opencv-python-headless

# Copy application code
COPY . .

# Set environment variables
ENV PYTHONUNBUFFERED=1

# Install CUDA dependencies only if USE_CUDA is true
RUN if [ "$USE_CUDA" = "true" ]; then \
    apt-get update && \
    apt-get install -y --no-install-recommends \
    nvidia-cuda-toolkit \
    && rm -rf /var/lib/apt/lists/*; \
    fi

WORKDIR /app/data
# Create data directory and copy models if they exist
RUN mkdir -p /app/data && \
    if [ -f "best.pt" ]; then cp best.pt /app/data/; fi && \
    if [ -f "yolo11x-cls.pt" ]; then cp yolo11x-cls.pt /app/data/; fi

CMD ["python3", "/app/main.py"] 