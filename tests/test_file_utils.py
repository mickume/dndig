"""Tests for file_utils module."""

import os
import tempfile
import pytest
from dndig.file_utils import (
    read_file_content,
    save_binary_file,
    ensure_directory_exists,
    validate_file_exists,
    sanitize_path,
    read_binary_file,
    validate_image_file,
    get_mime_type,
    resolve_reference_path,
)


class TestReadFileContent:
    """Tests for read_file_content function."""

    def test_read_existing_file(self):
        """Test reading an existing file."""
        with tempfile.NamedTemporaryFile(mode='w', delete=False) as f:
            f.write("test content")
            temp_path = f.name

        try:
            content = read_file_content(temp_path)
            assert content == "test content"
        finally:
            os.unlink(temp_path)

    def test_read_nonexistent_file_raises(self):
        """Test reading nonexistent file raises FileNotFoundError."""
        with pytest.raises(FileNotFoundError):
            read_file_content("/nonexistent/file.txt")


class TestSaveBinaryFile:
    """Tests for save_binary_file function."""

    def test_save_binary_data(self):
        """Test saving binary data to file."""
        with tempfile.NamedTemporaryFile(delete=False) as f:
            temp_path = f.name

        try:
            data = b"binary data here"
            save_binary_file(temp_path, data)

            # Read back and verify
            with open(temp_path, 'rb') as f:
                read_data = f.read()
            assert read_data == data
        finally:
            os.unlink(temp_path)


class TestEnsureDirectoryExists:
    """Tests for ensure_directory_exists function."""

    def test_create_new_directory(self):
        """Test creating a new directory."""
        with tempfile.TemporaryDirectory() as tmpdir:
            new_dir = os.path.join(tmpdir, "new_folder")
            ensure_directory_exists(new_dir)
            assert os.path.isdir(new_dir)

    def test_existing_directory_no_error(self):
        """Test that existing directory doesn't raise error."""
        with tempfile.TemporaryDirectory() as tmpdir:
            ensure_directory_exists(tmpdir)  # Should not raise
            assert os.path.isdir(tmpdir)


class TestValidateFileExists:
    """Tests for validate_file_exists function."""

    def test_validate_existing_file(self):
        """Test validating existing file."""
        with tempfile.NamedTemporaryFile(delete=False) as f:
            temp_path = f.name

        try:
            validate_file_exists(temp_path)  # Should not raise
        finally:
            os.unlink(temp_path)

    def test_validate_nonexistent_file_raises(self):
        """Test validating nonexistent file raises."""
        with pytest.raises(FileNotFoundError):
            validate_file_exists("/nonexistent/file.txt")


class TestSanitizePath:
    """Tests for sanitize_path function."""

    def test_sanitize_relative_path(self):
        """Test sanitizing relative path."""
        result = sanitize_path("./test.txt")
        assert os.path.isabs(result)

    def test_sanitize_with_base_dir_valid(self):
        """Test sanitizing path within base directory."""
        with tempfile.TemporaryDirectory() as tmpdir:
            test_path = os.path.join(tmpdir, "test.txt")
            result = sanitize_path(test_path, base_dir=tmpdir)
            assert result.startswith(tmpdir)

    def test_sanitize_with_base_dir_escape_raises(self):
        """Test that path escaping base directory raises."""
        with tempfile.TemporaryDirectory() as tmpdir:
            subdir = os.path.join(tmpdir, "subdir")
            os.makedirs(subdir)

            # Try to escape to parent
            escape_path = os.path.join(subdir, "..", "..", "escape.txt")
            with pytest.raises(ValueError):
                sanitize_path(escape_path, base_dir=subdir)


class TestReadBinaryFile:
    """Tests for read_binary_file function."""

    def test_read_binary_file(self):
        """Test reading binary file."""
        with tempfile.NamedTemporaryFile(mode='wb', delete=False) as f:
            test_data = b'\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR'
            f.write(test_data)
            temp_path = f.name

        try:
            data = read_binary_file(temp_path)
            assert data == test_data
        finally:
            os.unlink(temp_path)

    def test_read_binary_file_nonexistent_raises(self):
        """Test reading nonexistent binary file raises."""
        with pytest.raises(FileNotFoundError):
            read_binary_file("/nonexistent/image.jpg")


class TestValidateImageFile:
    """Tests for validate_image_file function."""

    def test_validate_valid_image_extensions(self):
        """Test validating files with valid image extensions."""
        valid_extensions = ['.jpg', '.jpeg', '.png', '.webp', '.gif']

        for ext in valid_extensions:
            with tempfile.NamedTemporaryFile(suffix=ext, delete=False) as f:
                temp_path = f.name

            try:
                validate_image_file(temp_path)  # Should not raise
            finally:
                os.unlink(temp_path)

    def test_validate_invalid_extension_raises(self):
        """Test validating file with invalid extension raises."""
        with tempfile.NamedTemporaryFile(suffix='.txt', delete=False) as f:
            temp_path = f.name

        try:
            with pytest.raises(ValueError, match="Unsupported image format"):
                validate_image_file(temp_path)
        finally:
            os.unlink(temp_path)

    def test_validate_nonexistent_file_raises(self):
        """Test validating nonexistent file raises FileNotFoundError."""
        with pytest.raises(FileNotFoundError):
            validate_image_file("/nonexistent/image.jpg")

    def test_validate_case_insensitive_extension(self):
        """Test that extension validation is case-insensitive."""
        with tempfile.NamedTemporaryFile(suffix='.JPG', delete=False) as f:
            temp_path = f.name

        try:
            validate_image_file(temp_path)  # Should not raise (uppercase .JPG)
        finally:
            os.unlink(temp_path)


class TestGetMimeType:
    """Tests for get_mime_type function."""

    def test_get_mime_type_jpg(self):
        """Test getting MIME type for JPG files."""
        with tempfile.NamedTemporaryFile(suffix='.jpg', delete=False) as f:
            temp_path = f.name

        try:
            mime_type = get_mime_type(temp_path)
            assert mime_type == 'image/jpeg'
        finally:
            os.unlink(temp_path)

    def test_get_mime_type_png(self):
        """Test getting MIME type for PNG files."""
        with tempfile.NamedTemporaryFile(suffix='.png', delete=False) as f:
            temp_path = f.name

        try:
            mime_type = get_mime_type(temp_path)
            assert mime_type == 'image/png'
        finally:
            os.unlink(temp_path)

    def test_get_mime_type_webp(self):
        """Test getting MIME type for WEBP files."""
        with tempfile.NamedTemporaryFile(suffix='.webp', delete=False) as f:
            temp_path = f.name

        try:
            mime_type = get_mime_type(temp_path)
            assert mime_type == 'image/webp'
        finally:
            os.unlink(temp_path)

    def test_get_mime_type_uppercase_extension(self):
        """Test getting MIME type with uppercase extension."""
        with tempfile.NamedTemporaryFile(suffix='.JPEG', delete=False) as f:
            temp_path = f.name

        try:
            mime_type = get_mime_type(temp_path)
            assert mime_type == 'image/jpeg'
        finally:
            os.unlink(temp_path)


class TestResolveReferencePath:
    """Tests for resolve_reference_path function."""

    def test_resolve_relative_path(self):
        """Test resolving relative path."""
        with tempfile.TemporaryDirectory() as tmpdir:
            base_dir = tmpdir
            relative_path = "assets/image.jpg"

            result = resolve_reference_path(relative_path, base_dir)
            expected = os.path.abspath(os.path.join(base_dir, "assets/image.jpg"))

            assert result == expected

    def test_resolve_absolute_path(self):
        """Test that absolute path is returned as-is."""
        abs_path = "/absolute/path/to/image.jpg"
        result = resolve_reference_path(abs_path, "/some/base/dir")

        assert result == abs_path

    def test_resolve_path_with_parent_reference(self):
        """Test resolving path with parent directory reference."""
        with tempfile.TemporaryDirectory() as tmpdir:
            base_dir = os.path.join(tmpdir, "prompts")
            relative_path = "../assets/image.jpg"

            result = resolve_reference_path(relative_path, base_dir)
            expected = os.path.abspath(os.path.join(base_dir, "../assets/image.jpg"))

            assert result == expected
