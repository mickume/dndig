"""Command-line interface for dndig."""

import argparse
import logging
import sys
from typing import Optional

from .generator import ImageGenerator, ImageGenerationError
from .api_client import GeminiAPIError
from .constants import DEFAULT_OUTPUT_DIR, MAX_CONCURRENT_WORKERS

logger = logging.getLogger(__name__)


def setup_logging(verbose: bool, debug: bool) -> None:
    """Configure logging based on verbosity level.

    Args:
        verbose: If True, set INFO level logging.
        debug: If True, set DEBUG level logging (overrides verbose).
    """
    if debug:
        level = logging.DEBUG
    elif verbose:
        level = logging.INFO
    else:
        level = logging.WARNING

    logging.basicConfig(
        level=level,
        format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
        datefmt='%Y-%m-%d %H:%M:%S',
    )


def create_parser() -> argparse.ArgumentParser:
    """Create and configure argument parser.

    Returns:
        Configured ArgumentParser instance.
    """
    parser = argparse.ArgumentParser(
        prog='dndig',
        description='Generate AI images using Google Gemini API',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  %(prog)s prompt.md
  %(prog)s prompt.md --verbose
  %(prog)s prompt.md --output-dir custom_art --workers 8
  %(prog)s prompt.md --debug

Environment Variables:
  GEMINI_API_KEY    Google Gemini API key (required)
        """,
    )

    parser.add_argument(
        'prompt_file',
        help='Path to markdown file with frontmatter and prompt',
    )

    parser.add_argument(
        '--output-dir',
        '-o',
        default=DEFAULT_OUTPUT_DIR,
        help=f'Output directory for generated images (default: {DEFAULT_OUTPUT_DIR})',
    )

    parser.add_argument(
        '--workers',
        '-w',
        type=int,
        default=MAX_CONCURRENT_WORKERS,
        help=f'Maximum concurrent API workers (default: {MAX_CONCURRENT_WORKERS})',
    )

    parser.add_argument(
        '--verbose',
        '-v',
        action='store_true',
        help='Enable verbose output with progress bar',
    )

    parser.add_argument(
        '--debug',
        action='store_true',
        help='Enable debug logging (most verbose)',
    )

    parser.add_argument(
        '--api-key',
        help='Google Gemini API key (overrides GEMINI_API_KEY env var)',
    )

    parser.add_argument(
        '--version',
        action='version',
        version='%(prog)s 2.0.0',
    )

    return parser


def main(argv: Optional[list] = None) -> int:
    """Main entry point for CLI.

    Args:
        argv: Optional command-line arguments (uses sys.argv if not provided).

    Returns:
        Exit code (0 for success, 1 for error).
    """
    parser = create_parser()
    args = parser.parse_args(argv)

    # Setup logging
    setup_logging(verbose=args.verbose, debug=args.debug)

    logger.debug(f"Arguments: {args}")

    try:
        # Create generator
        generator = ImageGenerator(
            output_dir=args.output_dir,
            max_workers=args.workers,
            api_key=args.api_key,
        )

        # Generate images
        generated_files = generator.generate_from_file(
            prompt_file=args.prompt_file,
            verbose=args.verbose,
        )

        # Print summary
        if not args.debug:  # Don't clutter debug output
            print(f"\nSuccess! Generated {len(generated_files)} image(s):")
            for file_path in generated_files:
                print(f"  - {file_path}")

        return 0

    except FileNotFoundError as e:
        logger.error(str(e))
        print(f"Error: {e}", file=sys.stderr)
        return 1

    except GeminiAPIError as e:
        logger.error(f"API error: {e}")
        print(f"API Error: {e}", file=sys.stderr)
        print("\nPlease check your GEMINI_API_KEY environment variable.", file=sys.stderr)
        return 1

    except ImageGenerationError as e:
        logger.error(f"Generation error: {e}")
        print(f"Error: {e}", file=sys.stderr)
        return 1

    except ValueError as e:
        logger.error(f"Configuration error: {e}")
        print(f"Configuration Error: {e}", file=sys.stderr)
        return 1

    except KeyboardInterrupt:
        logger.info("Interrupted by user")
        print("\nInterrupted by user", file=sys.stderr)
        return 130  # Standard exit code for SIGINT

    except Exception as e:
        logger.exception("Unexpected error")
        print(f"Unexpected error: {e}", file=sys.stderr)
        if args.debug:
            raise  # Re-raise in debug mode for full traceback
        return 1


if __name__ == '__main__':
    sys.exit(main())
