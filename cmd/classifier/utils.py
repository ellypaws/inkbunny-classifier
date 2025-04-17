import glob
import hashlib
import json
import os
import re

from Crypto.Cipher import AES


def get_latest_weights(last: bool = False) -> str:
    """
    Search for the best.pt file in the following order:
    1. Current working directory
    2. Inside directories matching runs/classify/train*/weights/
    Returns the best.pt file path, preferring the current directory.
    """
    # First check if best.pt exists in the current working directory
    cwd_weights = os.path.join(os.getcwd(), "last.pt" if last else "best.pt")
    if os.path.isfile(cwd_weights):
        print(f"Using weights from current directory: {cwd_weights}")
        return cwd_weights

    # Pattern: runs/classify/train*/weights/best.pt
    pattern = os.path.join("runs", "classify", "train*", "weights", "last.pt" if last else "best.pt")
    weight_files = glob.glob(pattern)
    if not weight_files:
        raise FileNotFoundError(f"No weights file found in current directory or {pattern}")

    # Extract the training number from the path (e.g., from 'train3')
    def extract_train_number(filepath) -> int:
        match = re.search(r"train(\d+)", filepath)
        # If there's no number (just "train"), return 0
        if not match:
            if "train/weights" in filepath:
                return 0
            return -1
        return int(match.group(1))

    # Sort the files by the training number and return the highest one
    weight_files.sort(key=extract_train_number)
    print(f"Using weights from: {weight_files[-1]}")
    return weight_files[-1]


def decrypt_aes_ctr(encrypted_data: bytes, key: str) -> bytes:
    """
    Decrypt the AES-CTR encrypted data.
    The first AES.block_size (16) bytes are assumed to be the IV.
    The encryption key is derived from the provided key string via SHA-256.
    """
    if not key:
        return encrypted_data
    derived_key = hashlib.sha256(key.encode()).digest()
    if len(encrypted_data) < AES.block_size:
        raise ValueError("Encrypted data is too short to contain an IV.")
    iv = encrypted_data[:AES.block_size]
    ciphertext = encrypted_data[AES.block_size:]
    iv_int = int.from_bytes(iv, byteorder='big')
    cipher = AES.new(derived_key, AES.MODE_CTR, nonce=b'', initial_value=iv_int)
    decrypted = cipher.decrypt(ciphertext)
    return decrypted


def save_file(data: bytes, predictions: dict[str, float], name: str) -> str:
    """
    Saves the provided data under the directory predict/{top_class}/name.
    If a file with the same name exists, appends _2, _3, ... to the filename.
    It also saves the predictions in a JSON file.
    Returns the final file path.
    """
    top_class = max(predictions, key=predictions.get)
    save_dir = os.path.join("predict", top_class)
    os.makedirs(save_dir, exist_ok=True)
    file_path = os.path.join(save_dir, name)
    json_path = os.path.join(save_dir, f"{name}.json")
    base, ext = os.path.splitext(name)
    counter = 2
    # Ensure unique filename if file already exists.
    while os.path.exists(file_path):
        file_path = os.path.join(save_dir, f"{base}_{counter}{ext}")
        json_path = os.path.join(save_dir, f"{base}_{counter}.json")
        counter += 1
    with open(file_path, "wb") as f:
        f.write(data)
    with open(json_path, "w") as json_file:
        json.dump(predictions, json_file)

    return file_path
