"""Tests for config module."""

import pytest
from dndig.config import GenerationConfig, parse_frontmatter


class TestParseFrontmatter:
    """Tests for parse_frontmatter function."""

    def test_parse_empty_content(self):
        """Test parsing content with no frontmatter."""
        content = "Just some text"
        frontmatter, body = parse_frontmatter(content)
        assert frontmatter == {}
        assert body == "Just some text"

    def test_parse_valid_frontmatter(self):
        """Test parsing valid frontmatter."""
        content = """---
title: test_image
aspect_ratio: 16:9
resolution: 2K
temperature: 0.8
batch: 4
---
This is the prompt text"""
        frontmatter, body = parse_frontmatter(content)
        assert frontmatter['title'] == 'test_image'
        assert frontmatter['aspect_ratio'] == '16:9'
        assert frontmatter['resolution'] == '2K'
        assert frontmatter['temperature'] == '0.8'
        assert frontmatter['batch'] == '4'
        assert body == 'This is the prompt text'

    def test_parse_frontmatter_with_quotes(self):
        """Test parsing frontmatter with quoted values."""
        content = """---
title: "quoted title"
aspect_ratio: '1:1'
---
Prompt"""
        frontmatter, body = parse_frontmatter(content)
        assert frontmatter['title'] == 'quoted title'
        assert frontmatter['aspect_ratio'] == '1:1'


class TestGenerationConfig:
    """Tests for GenerationConfig class."""

    def test_default_config(self):
        """Test default configuration values."""
        config = GenerationConfig()
        assert config.title == 'generated_image'
        assert config.aspect_ratio == '1:1'
        assert config.resolution == '1K'
        assert config.temperature == 1.0
        assert config.batch == 1
        assert config.instructions is None

    def test_config_validation_valid(self):
        """Test validation with valid values."""
        config = GenerationConfig(
            title='test',
            aspect_ratio='16:9',
            resolution='2K',
            temperature=0.5,
            batch=5,
        )
        config.validate()  # Should not raise

    def test_config_validation_invalid_aspect_ratio(self):
        """Test validation fails with invalid aspect ratio."""
        config = GenerationConfig(aspect_ratio='invalid')
        with pytest.raises(ValueError, match="Invalid aspect_ratio"):
            config.validate()

    def test_config_validation_invalid_resolution(self):
        """Test validation fails with invalid resolution."""
        config = GenerationConfig(resolution='8K')
        with pytest.raises(ValueError, match="Invalid resolution"):
            config.validate()

    def test_config_validation_invalid_temperature_high(self):
        """Test validation fails with temperature too high."""
        config = GenerationConfig(temperature=1.5)
        with pytest.raises(ValueError, match="Invalid temperature"):
            config.validate()

    def test_config_validation_invalid_temperature_low(self):
        """Test validation fails with temperature too low."""
        config = GenerationConfig(temperature=-0.5)
        with pytest.raises(ValueError, match="Invalid temperature"):
            config.validate()

    def test_config_validation_invalid_batch_size(self):
        """Test validation fails with invalid batch size."""
        config = GenerationConfig(batch=0)
        with pytest.raises(ValueError, match="Invalid batch size"):
            config.validate()

    def test_from_frontmatter_valid(self):
        """Test creating config from valid frontmatter."""
        frontmatter = {
            'title': 'my_image',
            'aspect_ratio': '16:9',
            'resolution': '4K',
            'temperature': '0.7',
            'batch': '3',
        }
        config = GenerationConfig.from_frontmatter(frontmatter)
        assert config.title == 'my_image'
        assert config.aspect_ratio == '16:9'
        assert config.resolution == '4K'
        assert config.temperature == 0.7
        assert config.batch == 3

    def test_from_frontmatter_with_defaults(self):
        """Test creating config with missing values uses defaults."""
        frontmatter = {'title': 'test'}
        config = GenerationConfig.from_frontmatter(frontmatter)
        assert config.title == 'test'
        assert config.aspect_ratio == '1:1'
        assert config.resolution == '1K'

    def test_from_frontmatter_invalid_raises(self):
        """Test creating config from invalid frontmatter raises."""
        frontmatter = {'temperature': 'not_a_number'}
        with pytest.raises(ValueError):
            GenerationConfig.from_frontmatter(frontmatter)
