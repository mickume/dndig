"""Configuration parsing and validation for dndig."""

import logging
from dataclasses import dataclass
from typing import Optional, Dict, Any, Tuple, List

from .constants import (
    DEFAULT_TITLE,
    DEFAULT_ASPECT_RATIO,
    DEFAULT_RESOLUTION,
    DEFAULT_TEMPERATURE,
    DEFAULT_BATCH_SIZE,
    VALID_ASPECT_RATIOS,
    VALID_RESOLUTIONS,
    MIN_TEMPERATURE,
    MAX_TEMPERATURE,
    MIN_BATCH_SIZE,
    MAX_BATCH_SIZE,
    MAX_REFERENCE_IMAGES,
)

logger = logging.getLogger(__name__)


@dataclass
class GenerationConfig:
    """Configuration for image generation."""

    title: str = DEFAULT_TITLE
    aspect_ratio: str = DEFAULT_ASPECT_RATIO
    resolution: str = DEFAULT_RESOLUTION
    temperature: float = DEFAULT_TEMPERATURE
    batch: int = DEFAULT_BATCH_SIZE
    instructions: Optional[str] = None
    references: Optional[List[str]] = None

    def validate(self) -> None:
        """Validate all configuration values.

        Raises:
            ValueError: If any configuration value is invalid.
        """
        # Validate aspect ratio
        if self.aspect_ratio not in VALID_ASPECT_RATIOS:
            raise ValueError(
                f"Invalid aspect_ratio '{self.aspect_ratio}'. "
                f"Must be one of: {', '.join(VALID_ASPECT_RATIOS)}"
            )

        # Validate resolution
        if self.resolution not in VALID_RESOLUTIONS:
            raise ValueError(
                f"Invalid resolution '{self.resolution}'. "
                f"Must be one of: {', '.join(VALID_RESOLUTIONS)}"
            )

        # Validate temperature
        if not (MIN_TEMPERATURE <= self.temperature <= MAX_TEMPERATURE):
            raise ValueError(
                f"Invalid temperature {self.temperature}. "
                f"Must be between {MIN_TEMPERATURE} and {MAX_TEMPERATURE}"
            )

        # Validate batch size
        if not (MIN_BATCH_SIZE <= self.batch <= MAX_BATCH_SIZE):
            raise ValueError(
                f"Invalid batch size {self.batch}. "
                f"Must be between {MIN_BATCH_SIZE} and {MAX_BATCH_SIZE}"
            )

        # Validate references
        if self.references:
            if len(self.references) > MAX_REFERENCE_IMAGES:
                raise ValueError(
                    f"Too many reference images ({len(self.references)}). "
                    f"Maximum is {MAX_REFERENCE_IMAGES}"
                )

        logger.debug(f"Configuration validated: {self}")

    @classmethod
    def from_frontmatter(cls, frontmatter: Dict[str, str]) -> 'GenerationConfig':
        """Create a GenerationConfig from parsed frontmatter.

        Args:
            frontmatter: Dictionary of frontmatter key-value pairs.

        Returns:
            GenerationConfig instance.

        Raises:
            ValueError: If any values fail validation or type conversion.
        """
        try:
            config = cls(
                title=frontmatter.get('title', DEFAULT_TITLE),
                aspect_ratio=frontmatter.get('aspect_ratio', DEFAULT_ASPECT_RATIO),
                resolution=frontmatter.get('resolution', DEFAULT_RESOLUTION),
                temperature=float(frontmatter.get('temperature', str(DEFAULT_TEMPERATURE))),
                batch=int(frontmatter.get('batch', str(DEFAULT_BATCH_SIZE))),
                instructions=frontmatter.get('instructions'),
                references=frontmatter.get('references'),
            )
            config.validate()
            return config
        except ValueError as e:
            raise ValueError(f"Invalid frontmatter configuration: {e}")


def parse_list_value(value: str) -> List[str]:
    """Parse YAML-style list from frontmatter value.

    Supports: [item1, item2] or single item.

    Args:
        value: String value from frontmatter.

    Returns:
        List of string items.
    """
    # Strip whitespace
    value = value.strip()

    # Handle list format [item1, item2]
    if value.startswith('[') and value.endswith(']'):
        items = value[1:-1].split(',')
        return [item.strip().strip('"').strip("'") for item in items if item.strip()]

    # Single item
    return [value.strip('"').strip("'")] if value else []


def parse_frontmatter(content: str) -> Tuple[Dict[str, Any], str]:
    """Parse YAML frontmatter from markdown content.

    Args:
        content: Markdown content potentially containing YAML frontmatter.

    Returns:
        Tuple of (frontmatter_dict, body_text).
    """
    frontmatter: Dict[str, Any] = {}
    body = content

    # Check if content starts with ---
    if content.startswith('---\n'):
        # Find the closing ---
        parts = content.split('---\n', 2)
        if len(parts) >= 3:
            frontmatter_text = parts[1]
            body = parts[2]

            # Parse frontmatter (simple key: value parsing)
            for line in frontmatter_text.strip().split('\n'):
                if ':' in line:
                    key, value = line.split(':', 1)
                    key_stripped = key.strip()

                    # Special handling for references (list field)
                    if key_stripped == 'references':
                        frontmatter[key_stripped] = parse_list_value(value)
                    else:
                        frontmatter[key_stripped] = value.strip().strip('"').strip("'")

    logger.debug(f"Parsed frontmatter: {frontmatter}")
    return frontmatter, body.strip()
