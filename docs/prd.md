# Product Requirements Document: Deneir

## 1. Product Overview

**Product Name:** Deneir

**Version:** 1.0

**Purpose:** Deneir is a command-line image generation tool that leverages Google's Generative AI (Gemini) to create high-quality images from markdown-formatted prompts with configurable parameters.

## 2. Problem Statement

Users need a simple, flexible way to generate AI images with fine-grained control over generation parameters without navigating complex web interfaces or APIs directly. Current solutions often lack:
- Easy configuration management
- Template-based prompt organization
- Local file-based workflow integration
- Version control friendly prompt storage

## 3. Target Users

- Digital artists and designers exploring AI-generated imagery
- Content creators requiring custom image assets
- Developers integrating AI image generation into workflows
- Researchers experimenting with generative AI models

## 4. Core Features

### 4.1 Image Generation
- Generate images using Google's Gemini Pro Image Preview model
- Support for streaming content generation
- Automatic timestamp-based file naming
- Organized output to `artwork/` directory

### 4.2 Prompt Management
- Markdown-based prompt files with YAML frontmatter
- Template system for consistent prompt structure
- Version control friendly text format
- Separation of prompt content and configuration

### 4.3 Configurable Parameters

**Aspect Ratio:**
- Supported ratios: 1:1, 2:3, 3:2, 4:5, 5:4, 9:16, 16:9, and more
- Default: 1:1

**Resolution:**
- Options: 1K, 2K, 4K
- Default: 1K

**Temperature:**
- Range: 0.0 to 1.0
- Controls creativity/randomness in generation
- Default: 1.0

**System Instructions:**
- Optional instruction files for model guidance
- Referenced via file path in frontmatter

### 4.4 Command-Line Interface
```bash
python create.py <prompt_file.md>
```

## 5. Technical Requirements

### 5.1 Dependencies
- Python 3.13+
- Google Generative AI SDK (google-genai)
- Valid Gemini API key (via `GEMINI_API_KEY` environment variable)

### 5.2 File Structure
```
deneir/
├── create.py              # Main generation script
├── prompts/
│   └── template.md        # Prompt template
├── artwork/               # Generated images output
└── requirements.txt       # Python dependencies
```

### 5.3 Prompt File Format
```markdown
---
title: image_name
aspect_ratio: "16:9"
resolution: 2K
temperature: 1.0
instructions: optional_instructions.md
---
Your detailed image generation prompt here
```

## 6. User Workflow

1. **Setup:** Install dependencies and configure Gemini API key
2. **Create Prompt:** Copy template and customize frontmatter and prompt content
3. **Generate:** Run `python create.py prompts/your_prompt.md`
4. **Retrieve:** Find generated image in `artwork/` directory with timestamp

## 7. Success Metrics

- Successful image generation rate
- User satisfaction with parameter control
- Ease of prompt management and organization
- Integration into existing creative workflows

## 8. Future Considerations

- Support for batch image generation
- Image variation generation from existing outputs
- Advanced parameter presets and profiles
- Output format options (PNG, JPEG)

## 9. Constraints and Limitations

- Requires active internet connection
- Dependent on Google Generative AI service availability
- Subject to Gemini API rate limits and quotas
- API costs based on Google's pricing model

## 10. Security and Privacy

- API key management via environment variables
- No logging of sensitive prompt content
- Local storage of generated images
- User responsible for prompt content compliance with Google's policies
