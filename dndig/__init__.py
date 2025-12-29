"""dndig: AI image generation CLI using Google Gemini API."""

__version__ = '2.0.0'
__author__ = 'dndig Contributors'
__license__ = 'MIT'

from .api_client import GeminiClient, GeminiAPIError
from .config import GenerationConfig, parse_frontmatter
from .generator import ImageGenerator, ImageGenerationError
from .cli import main

__all__ = [
    'GeminiClient',
    'GeminiAPIError',
    'GenerationConfig',
    'parse_frontmatter',
    'ImageGenerator',
    'ImageGenerationError',
    'main',
]
