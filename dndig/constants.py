"""Constants and default values for dndig."""

# API Configuration
GEMINI_MODEL = "gemini-3-pro-image-preview"
API_KEY_ENV_VAR = "GEMINI_API_KEY"

# Generation Defaults
DEFAULT_TITLE = "generated_image"
DEFAULT_ASPECT_RATIO = "1:1"
DEFAULT_RESOLUTION = "1K"
DEFAULT_TEMPERATURE = 1.0
DEFAULT_BATCH_SIZE = 1
DEFAULT_OUTPUT_DIR = "artwork"

# Concurrency
MAX_CONCURRENT_WORKERS = 4
DEFAULT_MAX_WORKERS = 4

# Validation Lists
VALID_ASPECT_RATIOS = ["16:9", "9:16", "1:1", "4:3", "3:4"]
VALID_RESOLUTIONS = ["1K", "2K", "4K"]

# Temperature Bounds
MIN_TEMPERATURE = 0.0
MAX_TEMPERATURE = 1.0

# Batch Limits
MIN_BATCH_SIZE = 1
MAX_BATCH_SIZE = 4  # Reasonable upper limit

# Response Configuration
RESPONSE_MODALITIES = ["IMAGE", "TEXT"]

# Reference Images
MAX_REFERENCE_IMAGES = 14  # Gemini 3 Pro Image Preview limit
SUPPORTED_IMAGE_FORMATS = {'.jpg', '.jpeg', '.png', '.webp', '.gif'}
