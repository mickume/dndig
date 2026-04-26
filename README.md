# dndig

AI image generation CLI powered by Google Gemini API.

dndig uses Markdown prompt files with YAML frontmatter to generate images. It supports batch generation with parallel workers, reference images for style guidance, and system instructions for consistent output across runs.

## Installation

Requires Python 3.10+ and a [Google Gemini API key](https://aistudio.google.com/apikey).

### Install directly from GitHub

```bash
uv pip install git+https://github.com/mickume/dndig.git
```

Or with pip:

```bash
pip install git+https://github.com/mickume/dndig.git
```

This installs the `dndig` command globally (in your active environment) without cloning the repo.

### Install from a local clone

```bash
git clone https://github.com/mickume/dndig.git
cd dndig
uv venv
source .venv/bin/activate
uv pip install -e .
```

To include development tools (pytest, black, flake8, mypy):

```bash
uv pip install -e ".[dev]"
```

### Set your API key

```bash
export GEMINI_API_KEY="your-api-key-here"
```

You can also pass it per-invocation with `--api-key`, or add the export to your shell profile.

## Quick start

1. Create a prompt file `sunset.md`:

   ```markdown
   ---
   title: mountain_sunset
   aspect_ratio: "16:9"
   resolution: 2K
   batch: 2
   ---
   A mountain landscape at golden hour with dramatic clouds and warm light
   ```

2. Generate images:

   ```bash
   dndig sunset.md --verbose
   ```

   Images are saved to the `artwork/` directory by default.

## Usage

```
dndig <prompt_file> [options]
```

| Option | Description |
|--------|-------------|
| `-o, --output-dir DIR` | Output directory (default: `artwork`) |
| `-w, --workers N` | Max concurrent API workers (default: 4) |
| `-v, --verbose` | Show progress bar |
| `--debug` | Enable debug logging |
| `--api-key KEY` | API key (overrides `GEMINI_API_KEY` env var) |
| `--version` | Show version |

### Examples

```bash
# Single image with defaults
dndig prompts/landscape.md

# Batch generation with progress bar
dndig prompts/batch.md --verbose

# Custom output directory and more workers
dndig prompts/portrait.md -o renders -w 8 --verbose

# Debug logging for troubleshooting
dndig prompts/test.md --debug
```

## Prompt file format

Prompt files are Markdown documents with a YAML frontmatter header. The frontmatter configures generation parameters; everything after the `---` block is the prompt text sent to the API.

```markdown
---
title: fantasy_castle
aspect_ratio: "16:9"
resolution: 2K
temperature: 0.8
batch: 4
instructions: style.md
references: [assets/castle_ref.jpg, assets/mountains_ref.png]
---
A majestic fantasy castle on a mountain peak at sunset with dramatic
lighting, detailed stonework, and mist around the base.
```

### Frontmatter options

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `title` | string | `generated_image` | Filename prefix for output images |
| `aspect_ratio` | string | `1:1` | `1:1`, `2:3`, `3:2`, `3:4`, `4:3`, `4:5`, `5:4`, `9:16`, `16:9`, `21:9` |
| `resolution` | string | `1K` | `512px`, `1K`, `2K`, `4K` |
| `temperature` | float | `1.0` | Creativity level, `0.0` to `1.0` |
| `batch` | int | `1` | Number of images to generate (max 4) |
| `instructions` | string | — | Path to a system instructions file |
| `references` | list | — | Paths to reference images (max 14) |

All paths in frontmatter (`instructions`, `references`) resolve relative to the prompt file's directory. Absolute paths also work.

### System instructions

The `instructions` field points to a plain text file containing style or behavioral directives applied to every generation. This is useful for maintaining a consistent visual style across prompts.

Example `style.md`:

```
Dramatic cinematic lighting with a blend of soft edges and painterly
brushwork. Highly saturated colors dominated by electric blues, magentas,
and warm golden oranges. Do not add descriptive text to the picture.
```

### Reference images

Reference images provide visual examples to guide the generation. They're useful for style transfer, composition guidance, or incorporating specific visual elements.

- Up to 14 images per generation
- Supported formats: JPG, JPEG, PNG, WEBP, GIF
- Use YAML list syntax: `[image1.jpg, image2.png]`

## Using as a library

```python
from dndig import ImageGenerator, GenerationConfig

generator = ImageGenerator(
    output_dir="my_output",
    max_workers=4,
    api_key="your-api-key"
)

images = generator.generate_from_file("prompt.md", verbose=True)
```

## Development

```bash
# Install with dev dependencies
uv pip install -e ".[dev]"

# Run tests
pytest

# Run with coverage
pytest --cov=dndig --cov-report=html

# Format, lint, type-check
black dndig/ tests/
flake8 dndig/ tests/
mypy dndig/
```

## Project structure

```
dndig/
├── dndig/
│   ├── __init__.py        # Package exports
│   ├── api_client.py      # Gemini API wrapper
│   ├── cli.py             # CLI entry point
│   ├── config.py          # Config parsing & validation
│   ├── constants.py       # Defaults and validation rules
│   ├── file_utils.py      # File I/O utilities
│   └── generator.py       # Image generation orchestration
├── tests/                 # Test suite
├── prompts/               # Example prompt files
│   ├── template.md        # Prompt template with all options
│   └── style.md           # Example style instructions
└── artwork/               # Generated images (git-ignored)
```

## License

MIT — see [LICENSE](LICENSE).
