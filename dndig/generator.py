"""Image generation orchestration for dndig."""

import logging
import mimetypes
import os
import threading
from concurrent.futures import ThreadPoolExecutor, as_completed
from dataclasses import asdict
from datetime import datetime
from typing import Optional, List

from tqdm import tqdm

from .api_client import GeminiClient, GeminiAPIError
from .config import GenerationConfig, parse_frontmatter
from .file_utils import (
    read_file_content,
    save_binary_file,
    ensure_directory_exists,
    validate_file_exists,
    save_generation_metadata,
)
from .constants import DEFAULT_OUTPUT_DIR, MAX_CONCURRENT_WORKERS

logger = logging.getLogger(__name__)


class ImageGenerationError(Exception):
    """Exception raised for image generation errors."""
    pass


class ImageGenerator:
    """Orchestrates image generation from prompts."""

    def __init__(
        self,
        output_dir: str = DEFAULT_OUTPUT_DIR,
        max_workers: int = MAX_CONCURRENT_WORKERS,
        api_key: Optional[str] = None,
    ):
        """Initialize image generator.

        Args:
            output_dir: Directory where images will be saved.
            max_workers: Maximum number of concurrent API workers.
            api_key: Optional API key (reads from environment if not provided).

        Raises:
            GeminiAPIError: If API client initialization fails.
        """
        self.output_dir = output_dir
        self.max_workers = max_workers
        self.client = GeminiClient(api_key=api_key)

        # Ensure output directory exists
        ensure_directory_exists(self.output_dir)

        logger.info(
            f"ImageGenerator initialized: output_dir={output_dir}, "
            f"max_workers={max_workers}"
        )

    def generate_from_file(
        self,
        prompt_file: str,
        verbose: bool = False,
    ) -> List[str]:
        """Generate images from a prompt file.

        Args:
            prompt_file: Path to markdown file with frontmatter and prompt.
            verbose: If True, show progress bar.

        Returns:
            List of paths to generated image files.

        Raises:
            FileNotFoundError: If prompt file doesn't exist.
            ImageGenerationError: If generation fails.
        """
        # Validate prompt file exists
        validate_file_exists(prompt_file)

        # Read and parse prompt
        try:
            prompt_content = read_file_content(prompt_file)
            frontmatter, prompt_text = parse_frontmatter(prompt_content)
            config = GenerationConfig.from_frontmatter(frontmatter)
        except Exception as e:
            raise ImageGenerationError(f"Failed to parse prompt file: {e}")

        # Read system instructions if specified
        system_instructions = None
        if config.instructions:
            try:
                validate_file_exists(config.instructions)
                system_instructions = read_file_content(config.instructions)
                logger.info(f"Loaded system instructions from {config.instructions}")
            except FileNotFoundError:
                logger.warning(
                    f"Instructions file not found: {config.instructions}. "
                    "Continuing without system instructions."
                )

        # Generate images
        logger.info(
            f"Starting generation: title={config.title}, batch={config.batch}"
        )

        generated_files = self._generate_batch(
            prompt_text=prompt_text,
            config=config,
            system_instructions=system_instructions,
            verbose=verbose,
        )

        # Save metadata
        timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
        save_generation_metadata(
            output_dir=self.output_dir,
            title=config.title,
            timestamp=timestamp,
            prompt_file=prompt_file,
            config_dict=asdict(config),
            image_files=generated_files,
        )

        logger.info(f"Generation complete: {len(generated_files)} image(s) created")
        return generated_files

    def _generate_batch(
        self,
        prompt_text: str,
        config: GenerationConfig,
        system_instructions: Optional[str],
        verbose: bool,
    ) -> List[str]:
        """Generate a batch of images using parallel workers.

        Args:
            prompt_text: The prompt text.
            config: Generation configuration.
            system_instructions: Optional system instructions.
            verbose: If True, show progress bar.

        Returns:
            List of paths to generated files.

        Raises:
            ImageGenerationError: If no images were generated.
        """
        # Generate timestamp for unique filenames
        timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")

        # Thread-safe counters and locks
        file_index = 1
        images_saved = 0
        generated_files: List[str] = []
        lock = threading.Lock()

        # Progress bar setup
        if verbose and config.batch > 1:
            progress_bar = tqdm(total=config.batch, desc="Generating images", unit="img")
        else:
            progress_bar = None

        # Worker function to make one API call and save images
        def generate_images_worker():
            nonlocal file_index, images_saved

            try:
                for chunk in self.client.generate_image_stream(
                    prompt=prompt_text,
                    config=config,
                    system_instructions=system_instructions,
                ):
                    # Check if chunk has image data
                    if (
                        chunk.candidates is None
                        or chunk.candidates[0].content is None
                        or chunk.candidates[0].content.parts is None
                    ):
                        continue

                    part = chunk.candidates[0].content.parts[0]
                    if part.inline_data and part.inline_data.data:
                        with lock:
                            # Stop if we've reached the desired batch count
                            if images_saved >= config.batch:
                                return

                            current_index = file_index
                            file_index += 1
                            images_saved += 1

                        # Save the image
                        file_name = f"{config.title}_{timestamp}_{current_index}"
                        inline_data = part.inline_data
                        data_buffer = inline_data.data
                        file_extension = mimetypes.guess_extension(inline_data.mime_type)
                        file_path = os.path.join(
                            self.output_dir, f"{file_name}{file_extension}"
                        )

                        save_binary_file(file_path, data_buffer)

                        with lock:
                            generated_files.append(file_path)
                            if progress_bar:
                                progress_bar.update(1)

            except GeminiAPIError as e:
                logger.error(f"API error in worker: {e}")
                raise
            except Exception as e:
                logger.error(f"Unexpected error in worker: {e}")
                raise

        # Use ThreadPoolExecutor to parallelize API calls
        max_workers = min(self.max_workers, config.batch)
        logger.debug(f"Using {max_workers} workers for batch of {config.batch}")

        try:
            with ThreadPoolExecutor(max_workers=max_workers) as executor:
                futures = []

                # Submit initial batch of workers
                for _ in range(max_workers):
                    if images_saved < config.batch:
                        futures.append(executor.submit(generate_images_worker))

                # Keep submitting new workers as old ones complete
                for future in as_completed(futures):
                    try:
                        future.result()  # Wait for completion and catch exceptions
                    except Exception as e:
                        logger.error(f"Worker failed: {e}")
                        # Continue with other workers

                    with lock:
                        if images_saved < config.batch:
                            # Submit another worker if we need more images
                            futures.append(executor.submit(generate_images_worker))

        finally:
            if progress_bar:
                progress_bar.close()

        # Check if any images were generated
        if images_saved == 0:
            raise ImageGenerationError("No images were generated")

        return generated_files
