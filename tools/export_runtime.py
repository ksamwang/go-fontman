from __future__ import annotations

import argparse
import json
import math
import shutil
import struct
from pathlib import Path

import numpy as np
import torch
from torch import nn
import torch.nn.functional as F


IMAGE_WIDTH = 384
IMAGE_HEIGHT = 128
EMBED_DIM = 512
RUNTIME_VERSION = 1
INDEX_MAGIC = b"FMIXF32\0"


class SmallFontCNN(nn.Module):
    def __init__(self, embed_dim: int = EMBED_DIM) -> None:
        super().__init__()
        self.features = nn.Sequential(
            block(3, 32, stride=2),
            block(32, 64, stride=2),
            block(64, 128, stride=2),
            block(128, 256, stride=2),
            block(256, 384, stride=2),
        )
        self.pool = nn.AdaptiveAvgPool2d(1)
        self.proj = nn.Linear(384, embed_dim)

    def forward(self, x):
        x = self.features(x)
        x = self.pool(x).flatten(1)
        x = self.proj(x)
        return F.normalize(x, dim=1, eps=1e-6)


def block(in_channels: int, out_channels: int, stride: int):
    return nn.Sequential(
        nn.Conv2d(in_channels, out_channels, 3, stride=stride, padding=1, bias=False),
        nn.BatchNorm2d(out_channels),
        nn.SiLU(inplace=True),
        nn.Conv2d(out_channels, out_channels, 3, stride=1, padding=1, bias=False),
        nn.BatchNorm2d(out_channels),
        nn.SiLU(inplace=True),
    )


class ArcFaceHead(nn.Module):
    def __init__(self, embed_dim: int, num_classes: int, scale: float = 30.0, margin: float = 0.35) -> None:
        super().__init__()
        self.weight = nn.Parameter(torch.empty(num_classes, embed_dim))
        nn.init.xavier_uniform_(self.weight)
        self.scale = scale
        self.margin = margin
        self.cos_m = math.cos(margin)
        self.sin_m = math.sin(margin)
        self.threshold = math.cos(math.pi - margin)
        self.mm = math.sin(math.pi - margin) * margin

    def forward(self, embeddings, labels):
        embeddings = embeddings.float()
        weight = self.weight.float()
        cosine = F.linear(F.normalize(embeddings, eps=1e-6), F.normalize(weight, eps=1e-6))
        cosine = cosine.clamp(-1.0 + 1e-4, 1.0 - 1e-4)
        sine = torch.sqrt((1.0 - cosine.square()).clamp_min(1e-6))
        target = cosine * self.cos_m - sine * self.sin_m
        target = torch.where(cosine > self.threshold, target, cosine - self.mm)
        one_hot = F.one_hot(labels, num_classes=cosine.size(1)).to(dtype=cosine.dtype, device=cosine.device)
        logits = cosine * (1.0 - one_hot) + target * one_hot
        return logits * self.scale


class FontEmbeddingModel(nn.Module):
    def __init__(self, num_classes: int, embed_dim: int = EMBED_DIM) -> None:
        super().__init__()
        self.backbone = SmallFontCNN(embed_dim)
        self.head = ArcFaceHead(embed_dim, num_classes)

    def forward(self, x, labels=None):
        embeddings = self.backbone(x)
        if labels is None:
            return embeddings
        return self.head(embeddings, labels)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Export go-fontman runtime artifacts from trained font_ai data.")
    parser.add_argument("--ai-dir", default="artifacts/font_ai/source", help="Directory containing font_embedding.pt, font_index.npz and font_index_meta.json.")
    parser.add_argument("--model-out", default="runtime/font_ai/model/font_embedding.onnx", help="Output ONNX model path.")
    parser.add_argument("--index-out", default="runtime/font_ai/data/font_index.f32bin", help="Output float32 vector index path.")
    parser.add_argument("--metadata-out", default="runtime/font_ai/data/font_index_meta.json", help="Output runtime metadata path.")
    parser.add_argument("--manifest-out", default="runtime/font_ai/manifest.json", help="Output runtime manifest path.")
    parser.add_argument("--opset", type=int, default=18, help="ONNX opset version.")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    ai_dir = Path(args.ai_dir)
    checkpoint_path = ai_dir / "font_embedding.pt"
    index_path = ai_dir / "font_index.npz"
    meta_path = ai_dir / "font_index_meta.json"
    train_config_path = ai_dir / "train_config.json"

    checkpoint = torch.load(checkpoint_path, map_location="cpu")
    model = FontEmbeddingModel(num_classes=int(checkpoint["num_classes"]), embed_dim=EMBED_DIM)
    model.load_state_dict(checkpoint["model"])
    model.backbone.eval()

    model_out = Path(args.model_out)
    model_out.parent.mkdir(parents=True, exist_ok=True)
    dummy = torch.randn(1, 3, IMAGE_HEIGHT, IMAGE_WIDTH, dtype=torch.float32)
    torch.onnx.export(
        model.backbone,
        dummy,
        model_out,
        input_names=["image"],
        output_names=["embedding"],
        dynamic_axes={"image": {0: "batch"}, "embedding": {0: "batch"}},
        opset_version=args.opset,
    )

    index = np.load(index_path)
    embeddings = np.asarray(index["embeddings"], dtype=np.float32)
    if embeddings.ndim != 2:
        raise ValueError(f"expected 2D embeddings, got shape {embeddings.shape}")
    index_out = Path(args.index_out)
    index_out.parent.mkdir(parents=True, exist_ok=True)
    write_f32_index(index_out, embeddings)

    metadata_out = Path(args.metadata_out)
    metadata_out.parent.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(meta_path, metadata_out)

    manifest = {
        "runtimeVersion": RUNTIME_VERSION,
        "source": {
            "checkpoint": str(checkpoint_path.as_posix()),
            "index": str(index_path.as_posix()),
            "metadata": str(meta_path.as_posix()),
            "trainConfig": str(train_config_path.as_posix()),
        },
        "model": {
            "path": str(model_out.as_posix()),
            "opset": args.opset,
            "inputName": "image",
            "inputShape": [1, 3, IMAGE_HEIGHT, IMAGE_WIDTH],
            "inputDtype": "float32",
            "outputName": "embedding",
            "embeddingDim": EMBED_DIM,
            "outputNormalized": True,
        },
        "preprocess": {
            "imageWidth": IMAGE_WIDTH,
            "imageHeight": IMAGE_HEIGHT,
            "colorMode": "RGB",
            "resize": "thumbnail_lanczos_keep_aspect",
            "canvasColor": [255, 255, 255],
            "normalize": "((pixel / 255.0) - 0.5) / 0.5",
            "layout": "NCHW",
        },
        "index": {
            "path": str(index_out.as_posix()),
            "format": "fontman-f32bin",
            "magic": INDEX_MAGIC.decode("ascii", errors="replace"),
            "count": int(embeddings.shape[0]),
            "dim": int(embeddings.shape[1]),
            "metric": "dot",
            "vectorsNormalized": True,
        },
        "metadata": {
            "path": str(metadata_out.as_posix()),
        },
    }
    manifest_out = Path(args.manifest_out)
    manifest_out.parent.mkdir(parents=True, exist_ok=True)
    manifest_out.write_text(json.dumps(manifest, ensure_ascii=False, indent=2), encoding="utf-8")

    print(f"exported ONNX model: {model_out}")
    print(f"exported vector index: {index_out} ({embeddings.shape[0]} x {embeddings.shape[1]})")
    print(f"exported metadata: {metadata_out}")
    print(f"exported manifest: {manifest_out}")


def write_f32_index(path: Path, embeddings: np.ndarray) -> None:
    data = np.ascontiguousarray(embeddings, dtype="<f4")
    with path.open("wb") as file:
        file.write(INDEX_MAGIC)
        file.write(struct.pack("<III", RUNTIME_VERSION, data.shape[0], data.shape[1]))
        file.write(data.tobytes(order="C"))


if __name__ == "__main__":
    main()
