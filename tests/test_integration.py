"""Integration tests for reference images feature."""

import os
import pytest
from dndig.config import parse_frontmatter, GenerationConfig
from dndig.file_utils import (
    read_file_content,
    resolve_reference_path,
    validate_image_file,
    get_mime_type,
    read_binary_file,
)
from dndig.generator import ImageGenerator


class TestReferenceImagesIntegration:
    """Integration tests for loading and processing reference images."""

    @pytest.fixture
    def test_prompt_file(self):
        """Path to test prompt with references."""
        return "prompts/test_with_references.md"

    def test_parse_prompt_with_references(self, test_prompt_file):
        """Test parsing a prompt file with references."""
        # Read and parse
        content = read_file_content(test_prompt_file)
        frontmatter, body = parse_frontmatter(content)

        # Verify frontmatter
        assert 'references' in frontmatter
        assert isinstance(frontmatter['references'], list)
        assert len(frontmatter['references']) == 2
        assert 'assets/castle_ref.jpg' in frontmatter['references']
        assert 'assets/style_ref.png' in frontmatter['references']

        # Verify body exists
        assert 'fantasy castle' in body.lower()

    def test_create_config_from_prompt_with_references(self, test_prompt_file):
        """Test creating GenerationConfig from prompt with references."""
        content = read_file_content(test_prompt_file)
        frontmatter, _ = parse_frontmatter(content)

        config = GenerationConfig.from_frontmatter(frontmatter)

        assert config.title == 'fantasy_scene'
        assert config.aspect_ratio == '16:9'
        assert config.resolution == '1K'
        assert config.batch == 2
        assert config.references is not None
        assert len(config.references) == 2

        # Should validate successfully
        config.validate()

    def test_load_reference_images(self, test_prompt_file):
        """Test loading reference images from prompt."""
        # Parse config
        content = read_file_content(test_prompt_file)
        frontmatter, _ = parse_frontmatter(content)
        config = GenerationConfig.from_frontmatter(frontmatter)

        # Get base directory
        base_dir = os.path.dirname(os.path.abspath(test_prompt_file))

        # Load each reference
        loaded_images = []
        for ref_path in config.references:
            # Resolve path
            abs_path = resolve_reference_path(ref_path, base_dir)
            assert os.path.exists(abs_path)

            # Validate
            validate_image_file(abs_path)

            # Load
            data = read_binary_file(abs_path)
            mime_type = get_mime_type(abs_path)

            loaded_images.append((data, mime_type))

        # Verify we loaded 2 images
        assert len(loaded_images) == 2

        # Verify first is JPEG
        assert loaded_images[0][1] == 'image/jpeg'
        assert len(loaded_images[0][0]) > 0

        # Verify second is PNG
        assert loaded_images[1][1] == 'image/png'
        assert len(loaded_images[1][0]) > 0

    def test_image_generator_load_references(self, test_prompt_file):
        """Test ImageGenerator.load_reference_images method."""
        # Create generator
        generator = ImageGenerator()

        # Get config
        content = read_file_content(test_prompt_file)
        frontmatter, _ = parse_frontmatter(content)
        config = GenerationConfig.from_frontmatter(frontmatter)

        # Load references using generator method
        base_dir = os.path.dirname(os.path.abspath(test_prompt_file))
        reference_images = generator.load_reference_images(
            config.references,
            base_dir
        )

        # Verify
        assert len(reference_images) == 2
        assert all(isinstance(item, tuple) for item in reference_images)
        assert all(len(item) == 2 for item in reference_images)
        assert all(isinstance(item[0], bytes) for item in reference_images)
        assert all(isinstance(item[1], str) for item in reference_images)

    def test_missing_reference_image_raises(self):
        """Test that missing reference image raises helpful error."""
        generator = ImageGenerator()

        with pytest.raises(Exception) as exc_info:
            generator.load_reference_images(
                ['nonexistent.jpg'],
                os.getcwd()
            )

        # Should mention the file and path resolution
        error_msg = str(exc_info.value)
        assert 'nonexistent.jpg' in error_msg
        assert 'not found' in error_msg.lower()

    def test_invalid_format_reference_raises(self, tmp_path):
        """Test that invalid image format raises helpful error."""
        # Create a text file with wrong extension
        invalid_file = tmp_path / "document.txt"
        invalid_file.write_text("not an image")

        generator = ImageGenerator()

        with pytest.raises(Exception) as exc_info:
            generator.load_reference_images(
                [str(invalid_file)],
                str(tmp_path)
            )

        # Should mention unsupported format
        error_msg = str(exc_info.value)
        assert 'document.txt' in error_msg or 'Unsupported' in error_msg or 'format' in error_msg.lower()


class TestInstructionsIntegration:
    """Integration tests for system instructions feature."""

    @pytest.fixture
    def instructions_file(self):
        """Path to test instructions file."""
        return "prompts/style.md"

    def test_instructions_file_exists(self, instructions_file):
        """Test that example instructions file exists."""
        assert os.path.exists(instructions_file)

    def test_load_instructions_content(self, instructions_file):
        """Test loading instructions file content."""
        content = read_file_content(instructions_file)

        # Verify it has content
        assert len(content) > 0

        # Verify it contains expected style instructions
        assert 'style' in content.lower() or 'color' in content.lower()

    def test_prompt_with_instructions(self, tmp_path):
        """Test creating a prompt with instructions field."""
        # Create instructions file
        instructions_path = tmp_path / "my_style.md"
        instructions_path.write_text("Fantasy art style with vibrant colors")

        # Create prompt file
        prompt_path = tmp_path / "test_prompt.md"
        prompt_path.write_text("""---
title: test_image
instructions: my_style.md
---
A fantasy landscape""")

        # Parse the prompt
        content = read_file_content(str(prompt_path))
        frontmatter, body = parse_frontmatter(content)

        # Create config
        config = GenerationConfig.from_frontmatter(frontmatter)

        assert config.instructions == 'my_style.md'
        assert body == 'A fantasy landscape'

    def test_prompt_with_instructions_and_references(self, tmp_path):
        """Test prompt with both instructions and references."""
        # Create instructions file
        instructions_path = tmp_path / "style.md"
        instructions_path.write_text("Use dramatic lighting")

        # Create reference images
        ref1 = tmp_path / "ref1.png"
        ref2 = tmp_path / "ref2.jpg"
        # Create minimal PNG
        ref1.write_bytes(b'\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x02\x00\x00\x00\x90wS\xde\x00\x00\x00\x0cIDATx\x9cc\xf8\xcf\xc0\x00\x00\x03\x01\x01\x00\x18\xdd\x8d\xb4\x00\x00\x00\x00IEND\xaeB`\x82')
        # Create minimal JPEG
        ref2.write_bytes(b'\xff\xd8\xff\xe0\x00\x10JFIF\x00\x01\x01\x00\x00\x01\x00\x01\x00\x00\xff\xdb\x00C\x00\x08\x06\x06\x07\x06\x05\x08\x07\x07\x07\t\t\x08\n\x0c\x14\r\x0c\x0b\x0b\x0c\x19\x12\x13\x0f\x14\x1d\x1a\x1f\x1e\x1d\x1a\x1c\x1c $(4,$&1\'-=-\x1c\x1c(7,01444\x1f\'9=82<.342\xff\xc0\x00\x0b\x08\x00\x01\x00\x01\x01\x01\x11\x00\xff\xc4\x00\x14\x00\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\xff\xc4\x00\x14\x10\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\xff\xda\x00\x08\x01\x01\x00\x00?\x00\x00\x00\xff\xd9')

        # Create prompt file
        prompt_path = tmp_path / "complete_prompt.md"
        prompt_path.write_text("""---
title: complete_test
instructions: style.md
references: [ref1.png, ref2.jpg]
---
Fantasy scene with references""")

        # Parse and validate
        content = read_file_content(str(prompt_path))
        frontmatter, body = parse_frontmatter(content)
        config = GenerationConfig.from_frontmatter(frontmatter)

        # Verify both fields are present
        assert config.instructions == 'style.md'
        assert config.references == ['ref1.png', 'ref2.jpg']

        # Validate config
        config.validate()

        # Verify instructions file can be loaded
        instructions_content = read_file_content(str(instructions_path))
        assert instructions_content == "Use dramatic lighting"

        # Verify reference images can be loaded
        generator = ImageGenerator()
        reference_images = generator.load_reference_images(
            config.references,
            str(tmp_path)
        )
        assert len(reference_images) == 2

    def test_missing_instructions_file_handling(self, tmp_path):
        """Test that missing instructions file is handled gracefully."""
        # Create prompt file with non-existent instructions
        prompt_path = tmp_path / "test.md"
        prompt_path.write_text("""---
title: test
instructions: nonexistent.md
---
Test prompt""")

        # Parse config
        content = read_file_content(str(prompt_path))
        frontmatter, _ = parse_frontmatter(content)
        config = GenerationConfig.from_frontmatter(frontmatter)

        # Config should be created successfully
        assert config.instructions == 'nonexistent.md'

        # Validation should pass (validation doesn't check file existence)
        config.validate()
