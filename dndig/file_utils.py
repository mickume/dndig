"""File I/O utilities for dndig."""

import json
import logging
import mimetypes
import os
import struct
from datetime import datetime
from typing import Optional, Tuple

from .constants import SUPPORTED_IMAGE_FORMATS, MIN_REFERENCE_IMAGE_DIMENSION

logger = logging.getLogger(__name__)


def read_file_content(file_path: str) -> str:
    """Read content from a text file.

    Args:
        file_path: Path to the file to read.

    Returns:
        File content as string.

    Raises:
        FileNotFoundError: If file does not exist.
        IOError: If file cannot be read.
    """
    try:
        with open(file_path, 'r', encoding='utf-8') as f:
            content = f.read()
        logger.debug(f"Read {len(content)} bytes from {file_path}")
        return content
    except FileNotFoundError:
        logger.error(f"File not found: {file_path}")
        raise
    except IOError as e:
        logger.error(f"Error reading file {file_path}: {e}")
        raise


def save_binary_file(file_path: str, data: bytes) -> None:
    """Save binary data to a file.

    Args:
        file_path: Path where file should be saved.
        data: Binary data to write.

    Raises:
        IOError: If file cannot be written.
    """
    try:
        with open(file_path, 'wb') as f:
            f.write(data)
        logger.info(f"File saved to: {file_path}")
    except IOError as e:
        logger.error(f"Error writing file {file_path}: {e}")
        raise


def ensure_directory_exists(directory: str) -> None:
    """Create directory if it doesn't exist.

    Args:
        directory: Path to directory.

    Raises:
        OSError: If directory cannot be created.
    """
    try:
        os.makedirs(directory, exist_ok=True)
        logger.debug(f"Ensured directory exists: {directory}")
    except OSError as e:
        logger.error(f"Error creating directory {directory}: {e}")
        raise


def validate_file_exists(file_path: str) -> None:
    """Validate that a file exists.

    Args:
        file_path: Path to file.

    Raises:
        FileNotFoundError: If file does not exist.
    """
    if not os.path.exists(file_path):
        raise FileNotFoundError(f"File not found: {file_path}")
    logger.debug(f"Validated file exists: {file_path}")


def save_generation_metadata(
    output_dir: str,
    title: str,
    timestamp: str,
    prompt_file: str,
    config_dict: dict,
    image_files: list,
) -> None:
    """Save metadata about the generation session.

    Args:
        output_dir: Directory where metadata should be saved.
        title: Title of the generation.
        timestamp: Timestamp string for the generation.
        prompt_file: Path to the prompt file used.
        config_dict: Configuration dictionary.
        image_files: List of generated image file paths.
    """
    metadata = {
        "title": title,
        "timestamp": timestamp,
        "prompt_file": prompt_file,
        "config": config_dict,
        "images": image_files,
        "generated_at": datetime.now().isoformat(),
    }

    metadata_file = os.path.join(output_dir, f"{title}_{timestamp}_metadata.json")

    try:
        with open(metadata_file, 'w', encoding='utf-8') as f:
            json.dump(metadata, f, indent=2)
        logger.info(f"Metadata saved to: {metadata_file}")
    except IOError as e:
        logger.warning(f"Failed to save metadata: {e}")


def sanitize_path(path: str, base_dir: Optional[str] = None) -> str:
    """Sanitize a file path to prevent path traversal attacks.

    Args:
        path: Path to sanitize.
        base_dir: Optional base directory to constrain path within.

    Returns:
        Sanitized absolute path.

    Raises:
        ValueError: If path tries to escape base_dir.
    """
    # Resolve to absolute path
    abs_path = os.path.abspath(path)

    # If base_dir specified, ensure path is within it
    if base_dir:
        abs_base = os.path.abspath(base_dir)
        # Check if path is within base directory
        if not abs_path.startswith(abs_base + os.sep) and abs_path != abs_base:
            raise ValueError(
                f"Path '{path}' is outside allowed directory '{base_dir}'"
            )

    logger.debug(f"Sanitized path: {path} -> {abs_path}")
    return abs_path


def read_binary_file(file_path: str) -> bytes:
    """Read binary content from a file.

    Args:
        file_path: Path to the file to read.

    Returns:
        File content as bytes.

    Raises:
        FileNotFoundError: If file does not exist.
        IOError: If file cannot be read.
    """
    sanitized_path = sanitize_path(file_path)
    logger.debug(f"Reading binary file: {sanitized_path}")
    try:
        with open(sanitized_path, 'rb') as f:
            return f.read()
    except FileNotFoundError:
        logger.error(f"File not found: {sanitized_path}")
        raise
    except IOError as e:
        logger.error(f"Error reading file {sanitized_path}: {e}")
        raise


def get_image_dimensions(file_path: str) -> Optional[Tuple[int, int]]:
    """Parse image dimensions from file headers without external dependencies.

    Supports PNG and JPEG. Returns None for other formats.
    """
    ext = os.path.splitext(file_path)[1].lower()
    try:
        with open(file_path, 'rb') as f:
            if ext == '.png':
                header = f.read(24)
                if len(header) >= 24 and header[:8] == b'\x89PNG\r\n\x1a\n':
                    w, h = struct.unpack('>II', header[16:24])
                    return (w, h)
            elif ext in ('.jpg', '.jpeg'):
                f.read(2)  # skip SOI marker
                while True:
                    marker, size = struct.unpack('>HH', f.read(4))
                    if marker in (0xFFC0, 0xFFC1, 0xFFC2):
                        f.read(1)  # skip precision byte
                        h, w = struct.unpack('>HH', f.read(4))
                        return (w, h)
                    f.read(size - 2)
    except Exception:
        pass
    return None


def validate_image_file(file_path: str) -> None:
    """Validate that a file exists, is a supported format, and meets minimum size.

    Args:
        file_path: Path to image file.

    Raises:
        FileNotFoundError: If file does not exist.
        ValueError: If file format is not supported or image is too small.
    """
    if not os.path.exists(file_path):
        raise FileNotFoundError(file_path)

    ext = os.path.splitext(file_path)[1].lower()
    if ext not in SUPPORTED_IMAGE_FORMATS:
        raise ValueError(
            f"Unsupported image format '{ext}'. "
            f"Supported formats: {', '.join(sorted(SUPPORTED_IMAGE_FORMATS))}"
        )

    dims = get_image_dimensions(file_path)
    if dims is not None:
        w, h = dims
        min_dim = MIN_REFERENCE_IMAGE_DIMENSION
        if w < min_dim or h < min_dim:
            raise ValueError(
                f"Image too small ({w}x{h}px). "
                f"Reference images must be at least {min_dim}x{min_dim}px."
            )

    logger.debug(f"Validated image file: {file_path}")


def get_mime_type(file_path: str) -> str:
    """Get MIME type for an image file.

    Args:
        file_path: Path to image file.

    Returns:
        MIME type string (e.g., 'image/jpeg').
    """
    mime_type, _ = mimetypes.guess_type(file_path)

    if not mime_type or not mime_type.startswith('image/'):
        ext = os.path.splitext(file_path)[1].lower()
        # Default mappings for common image formats
        defaults = {
            '.jpg': 'image/jpeg',
            '.jpeg': 'image/jpeg',
            '.png': 'image/png',
            '.webp': 'image/webp',
            '.gif': 'image/gif'
        }
        mime_type = defaults.get(ext, 'image/jpeg')

    logger.debug(f"MIME type for {file_path}: {mime_type}")
    return mime_type


def resolve_path(path: str, base_dir: str) -> str:
    """Resolve a file path relative to a base directory.

    All paths in frontmatter (instructions, references) are resolved
    relative to the prompt file's directory. Absolute paths are returned
    as-is.

    Args:
        path: Path from frontmatter (may be relative or absolute).
        base_dir: Base directory (typically prompt file's directory).

    Returns:
        Absolute path to the file.
    """
    if os.path.isabs(path):
        return path

    resolved = os.path.abspath(os.path.join(base_dir, path))
    logger.debug(f"Resolved path: {path} -> {resolved}")
    return resolved
