"""Tests for config module."""

import pytest
from dndig.config import GenerationConfig, parse_frontmatter, parse_list_value


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

    def test_parse_frontmatter_with_references_list(self):
        """Test parsing frontmatter with references list."""
        content = """---
title: test
references: [ref1.jpg, ref2.png, ref3.webp]
---
Prompt with references"""
        frontmatter, body = parse_frontmatter(content)
        assert frontmatter['title'] == 'test'
        assert frontmatter['references'] == ['ref1.jpg', 'ref2.png', 'ref3.webp']
        assert body == 'Prompt with references'

    def test_parse_frontmatter_with_single_reference(self):
        """Test parsing frontmatter with single reference."""
        content = """---
references: single_ref.jpg
---
Prompt"""
        frontmatter, body = parse_frontmatter(content)
        assert frontmatter['references'] == ['single_ref.jpg']


class TestParseListValue:
    """Tests for parse_list_value function."""

    def test_parse_list_format(self):
        """Test parsing list format [item1, item2]."""
        result = parse_list_value('[item1.jpg, item2.png, item3.webp]')
        assert result == ['item1.jpg', 'item2.png', 'item3.webp']

    def test_parse_list_with_quotes(self):
        """Test parsing list with quoted items."""
        result = parse_list_value('["item1.jpg", "item2.png"]')
        assert result == ['item1.jpg', 'item2.png']

    def test_parse_list_with_spaces(self):
        """Test parsing list with extra spaces."""
        result = parse_list_value('[  item1.jpg  ,  item2.png  ]')
        assert result == ['item1.jpg', 'item2.png']

    def test_parse_single_value(self):
        """Test parsing single value (not a list)."""
        result = parse_list_value('single_item.jpg')
        assert result == ['single_item.jpg']

    def test_parse_empty_list(self):
        """Test parsing empty list."""
        result = parse_list_value('[]')
        assert result == []

    def test_parse_empty_string(self):
        """Test parsing empty string."""
        result = parse_list_value('')
        assert result == []


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
        assert config.references is None

    def test_config_validation_valid(self):
        """Test validation with valid values."""
        config = GenerationConfig(
            title='test',
            aspect_ratio='16:9',
            resolution='2K',
            temperature=0.5,
            batch=4,  # Max batch size is 4
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

    def test_config_with_references(self):
        """Test creating config with references."""
        config = GenerationConfig(
            title='test',
            references=['ref1.jpg', 'ref2.png']
        )
        assert config.references == ['ref1.jpg', 'ref2.png']
        config.validate()  # Should not raise

    def test_config_validation_too_many_references(self):
        """Test validation fails with too many references."""
        # Create 15 references (max is 14)
        too_many_refs = [f'ref{i}.jpg' for i in range(15)]
        config = GenerationConfig(references=too_many_refs)
        with pytest.raises(ValueError, match="Too many reference images"):
            config.validate()

    def test_config_validation_max_references(self):
        """Test validation succeeds with exactly max references."""
        # Create exactly 14 references (the maximum)
        max_refs = [f'ref{i}.jpg' for i in range(14)]
        config = GenerationConfig(references=max_refs)
        config.validate()  # Should not raise

    def test_from_frontmatter_with_references(self):
        """Test creating config from frontmatter with references."""
        frontmatter = {
            'title': 'styled_image',
            'references': ['style.jpg', 'character.png']
        }
        config = GenerationConfig.from_frontmatter(frontmatter)
        assert config.title == 'styled_image'
        assert config.references == ['style.jpg', 'character.png']

    def test_config_with_instructions(self):
        """Test creating config with instructions path."""
        config = GenerationConfig(
            title='test',
            instructions='prompts/style.md'
        )
        assert config.instructions == 'prompts/style.md'
        config.validate()  # Should not raise

    def test_from_frontmatter_with_instructions(self):
        """Test creating config from frontmatter with instructions."""
        frontmatter = {
            'title': 'styled_image',
            'instructions': 'prompts/custom_style.md'
        }
        config = GenerationConfig.from_frontmatter(frontmatter)
        assert config.title == 'styled_image'
        assert config.instructions == 'prompts/custom_style.md'

    def test_from_frontmatter_with_both_instructions_and_references(self):
        """Test creating config with both instructions and references."""
        frontmatter = {
            'title': 'complete_config',
            'aspect_ratio': '16:9',
            'resolution': '2K',
            'temperature': '0.8',
            'batch': '2',
            'instructions': 'prompts/style.md',
            'references': ['ref1.jpg', 'ref2.png']
        }
        config = GenerationConfig.from_frontmatter(frontmatter)
        assert config.title == 'complete_config'
        assert config.aspect_ratio == '16:9'
        assert config.resolution == '2K'
        assert config.temperature == 0.8
        assert config.batch == 2
        assert config.instructions == 'prompts/style.md'
        assert config.references == ['ref1.jpg', 'ref2.png']
        config.validate()  # Should not raise

    def test_parse_frontmatter_with_instructions(self):
        """Test parsing frontmatter with instructions field."""
        content = """---
title: test_image
instructions: prompts/style.md
---
Test prompt"""
        frontmatter, body = parse_frontmatter(content)
        assert frontmatter['title'] == 'test_image'
        assert frontmatter['instructions'] == 'prompts/style.md'
        assert body == 'Test prompt'
