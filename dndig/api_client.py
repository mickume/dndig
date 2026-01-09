"""Google Gemini API client wrapper for dndig."""

import logging
import os
from typing import Optional, Iterator, Any, List, Tuple

from google import genai
from google.genai import types

from .config import GenerationConfig
from .constants import (
    GEMINI_MODEL,
    API_KEY_ENV_VAR,
    RESPONSE_MODALITIES,
)

logger = logging.getLogger(__name__)


class GeminiAPIError(Exception):
    """Exception raised for Gemini API errors."""
    pass


class GeminiClient:
    """Wrapper for Google Gemini API client."""

    def __init__(self, api_key: Optional[str] = None):
        """Initialize Gemini client.

        Args:
            api_key: API key for Gemini. If not provided, reads from environment.

        Raises:
            GeminiAPIError: If API key is not provided or found in environment.
        """
        self.api_key = api_key or os.environ.get(API_KEY_ENV_VAR)

        if not self.api_key:
            raise GeminiAPIError(
                f"API key not found. Set {API_KEY_ENV_VAR} environment variable "
                "or provide api_key parameter."
            )

        try:
            self.client = genai.Client(api_key=self.api_key)
            logger.info("Gemini API client initialized successfully")
        except Exception as e:
            raise GeminiAPIError(f"Failed to initialize Gemini client: {e}")

    def generate_image_stream(
        self,
        prompt: str,
        config: GenerationConfig,
        system_instructions: Optional[str] = None,
        reference_images: Optional[List[Tuple[bytes, str]]] = None,
    ) -> Iterator[Any]:
        """Generate an image using streaming API.

        Args:
            prompt: Text prompt for image generation.
            config: Generation configuration.
            system_instructions: Optional system instructions.
            reference_images: Optional list of (image_data, mime_type) tuples.

        Yields:
            Response chunks from the API.

        Raises:
            GeminiAPIError: If API call fails.
        """
        try:
            # Build parts list starting with text prompt
            parts = [types.Part.from_text(text=prompt)]

            # Add reference images if provided
            if reference_images:
                for img_data, mime_type in reference_images:
                    parts.append(types.Part.from_bytes(data=img_data, mime_type=mime_type))

            contents = [
                types.Content(
                    role="user",
                    parts=parts,
                ),
            ]

            tools = [types.Tool(googleSearch=types.GoogleSearch())]

            # Build config with optional system instructions
            config_params = {
                "temperature": config.temperature,
                "response_modalities": RESPONSE_MODALITIES,
                "image_config": types.ImageConfig(
                    image_size=config.resolution,
                    aspect_ratio=config.aspect_ratio,
                ),
                "tools": tools,
            }

            if system_instructions is not None:
                config_params["system_instruction"] = [
                    types.Part.from_text(text=system_instructions),
                ]

            generate_content_config = types.GenerateContentConfig(**config_params)

            logger.debug(
                f"Generating image with model={GEMINI_MODEL}, "
                f"temperature={config.temperature}, "
                f"aspect_ratio={config.aspect_ratio}, "
                f"resolution={config.resolution}, "
                f"reference_images={len(reference_images) if reference_images else 0}"
            )

            response_stream = self.client.models.generate_content_stream(
                model=GEMINI_MODEL,
                contents=contents,
                config=generate_content_config,
            )

            for chunk in response_stream:
                yield chunk

        except Exception as e:
            logger.error(f"API error during image generation: {e}")
            raise GeminiAPIError(f"Failed to generate image: {e}")

    def validate_connection(self) -> bool:
        """Validate that the API connection works.

        Returns:
            True if connection is valid, False otherwise.
        """
        try:
            # Simple check that client is initialized
            return self.client is not None
        except Exception as e:
            logger.error(f"Connection validation failed: {e}")
            return False
