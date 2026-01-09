# Dndig - Image creation tool

AI image generation CLI powered by Google Gemini API.

## Overview

dndig is a command-line tool for generating AI images using Google's Gemini API. It features a template-based workflow with YAML frontmatter configuration, parallel batch processing, and comprehensive error handling.

## Features

- **Template-based Prompts**: Use Markdown files with YAML frontmatter for configuration
- **Batch Generation**: Generate multiple images in parallel with configurable concurrency
- **Progress Tracking**: Visual progress bars for batch operations
- **Flexible Configuration**: Control aspect ratio, resolution, temperature, and more
- **System Instructions**: Apply custom style instructions across generations
- **Metadata Tracking**: Automatic metadata logging for reproducibility
- **Type-Safe**: Full type hints and validation
- **Comprehensive Testing**: Unit tests with pytest
- **Installable Package**: pip-installable with entry point

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/yourusername/dndig.git
cd dndig

# Create virtual environment
python313 -m venv venv
source venv/bin/activate  # On Windows: venv\Scripts\activate

# Install in development mode
pip install -e .

# Or install with dev dependencies
pip install -e ".[dev]"
```

### From PyPI (when published)

```bash
pip install dndig
```

## Quick Start

1. **Set your API key**:

   ```bash
   export GEMINI_API_KEY="your-api-key-here"
   ```

2. **Create a prompt file** (`my_prompt.md`):

   ```markdown
   ---
   title: mountain_sunset
   aspect_ratio: 16:9
   resolution: 2K
   temperature: 1.0
   batch: 4
   ---
   A beautiful mountain landscape at sunset with vibrant colors
   ```

3. **Generate images**:

   ```bash
   dndig my_prompt.md --verbose
   ```

## Usage

### Command Line Interface

```bash
dndig <prompt_file> [options]

Options:
  -o, --output-dir DIR    Output directory for images (default: artwork)
  -w, --workers N         Max concurrent workers (default: 4)
  -v, --verbose           Enable verbose output with progress bar
  --debug                 Enable debug logging
  --api-key KEY           API key (overrides GEMINI_API_KEY env var)
  --version              Show version and exit
  -h, --help             Show help message
```

### Prompt File Format

Prompt files use Markdown with YAML frontmatter:

```markdown
---
title: my_image                        # Output filename prefix (required)
aspect_ratio: "16:9"                   # Options: 16:9, 9:16, 1:1, 4:3, 3:4
resolution: 2K                         # Options: 1K, 2K, 4K
temperature: 1.0                       # Range: 0.0-1.0 (creativity level)
batch: 4                               # Number of images to generate
instructions: path/style.md            # Optional style instructions file
references: [ref1.jpg, ref2.png]       # Optional reference images (up to 14)
---

Your detailed prompt text goes here.
Describe the image you want to generate.
```

### Configuration Options

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `title` | string | `generated_image` | Filename prefix for outputs |
| `aspect_ratio` | string | `1:1` | Image aspect ratio |
| `resolution` | string | `1K` | Image resolution |
| `temperature` | float | `1.0` | Generation creativity (0.0-1.0) |
| `batch` | int | `1` | Number of images to generate |
| `instructions` | string | `null` | Path to system instructions file |
| `references` | list | `null` | List of reference image paths (max 14) |

#### Reference Images

Reference images help ground the generated images with visual examples. Paths are resolved relative to the prompt file's directory.

- **Maximum**: 14 reference images per generation
- **Supported formats**: JPG, JPEG, PNG, WEBP, GIF
- **Path resolution**: Relative paths are resolved from the prompt file's directory
- **Format**: Use YAML list syntax: `[image1.jpg, image2.png]`

Example:
```markdown
---
title: styled_portrait
references: [assets/style_ref.jpg, assets/face_ref.png]
---
Portrait incorporating the style and features from the reference images
```

### Examples

**Generate a single image:**

```bash
dndig prompts/landscape.md
```

**Generate with custom output directory:**

```bash
dndig prompts/portrait.md --output-dir custom_art
```

**Generate with more workers:**

```bash
dndig prompts/batch.md --workers 8 --verbose
```

**Debug mode:**

```bash
dndig prompts/test.md --debug
```

**Generate with reference images:**

```bash
# Create prompt with references
cat > prompts/styled_scene.md << 'EOF'
---
title: fantasy_castle
aspect_ratio: 16:9
resolution: 2K
references: [assets/castle_ref.jpg, assets/mountains_ref.png]
---
A majestic fantasy castle incorporating architectural elements from the references
EOF

# Generate
dndig prompts/styled_scene.md --verbose
```

## Project Structure

```
dndig/
├── dndig/               # Main package
│   ├── __init__.py       # Package initialization
│   ├── api_client.py     # Gemini API wrapper
│   ├── cli.py            # Command-line interface
│   ├── config.py         # Configuration & validation
│   ├── constants.py      # Constants and defaults
│   ├── file_utils.py     # File I/O utilities
│   └── generator.py      # Image generation orchestration
├── tests/                # Test suite
│   ├── test_config.py
│   ├── test_file_utils.py
│   ├── test_api_client.py
│   └── fixtures/
├── prompts/              # Example prompts
├── artwork/              # Generated images (git-ignored)
├── setup.py              # Package setup
├── requirements.txt      # Dependencies
└── README.md            # This file
```

## Development

### Running Tests

```bash
# Install dev dependencies
pip install -e ".[dev]"

# Run tests
pytest

# Run with coverage
pytest --cov=dndig --cov-report=html

# Run specific test file
pytest tests/test_config.py
```

### Code Quality

```bash
# Format code
black dndig/ tests/

# Lint code
flake8 dndig/ tests/

# Type checking
mypy dndig/
```

### Using as a Library

```python
from dndig import ImageGenerator, GenerationConfig

# Initialize generator
generator = ImageGenerator(
    output_dir="my_output",
    max_workers=4,
    api_key="your-api-key"
)

# Generate from file
images = generator.generate_from_file("prompt.md", verbose=True)

# Or programmatically
config = GenerationConfig(
    title="my_image",
    aspect_ratio="16:9",
    resolution="4K",
    temperature=1.0,
    batch=2
)
```

## Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `GEMINI_API_KEY` | Google Gemini API key | Yes |

Create a `.env` file (see `.env.example`):

```bash
GEMINI_API_KEY=your-api-key-here
```

## Troubleshooting

### "API key not found" error

Ensure `GEMINI_API_KEY` is set:

```bash
echo $GEMINI_API_KEY  # Should print your key
```

### "Invalid aspect_ratio" error

Check that aspect ratio is one of: `16:9`, `9:16`, `1:1`, `4:3`, `3:4`

### "Invalid resolution" error

Resolution must be: `1K`, `2K`, or `4K`

### Import errors after installation

Ensure you're in the correct virtual environment:

```bash
which python  # Should point to venv/bin/python
pip list | grep dndig  # Should show dndig package
```

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes with tests
4. Run tests and linting
5. Commit your changes (`git commit -m 'Add amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Changelog

### v2.0.0 (2024-12-29)

- Complete refactoring into modular package structure
- Added comprehensive error handling and validation
- Added type hints throughout codebase
- Added progress bars for batch operations
- Added logging with configurable levels
- Added unit tests with pytest
- Made package pip-installable
- Added CLI with argparse
- Added metadata tracking
- Improved documentation

### v1.0.0

- Initial release with basic batch and parallel processing

## Acknowledgments

- Powered by [Google Gemini API](https://ai.google.dev/)
- Built with Python 3.8+