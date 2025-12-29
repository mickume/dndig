import base64
import mimetypes
import os
import re
import sys
from datetime import datetime
from google import genai
from google.genai import types


def parse_frontmatter(content):
    """Parse YAML frontmatter from markdown content."""
    frontmatter = {}
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
                    frontmatter[key.strip()] = value.strip().strip('"').strip("'")
    
    return frontmatter, body.strip()


def read_file_content(file_path):
    """Read content from a file."""
    with open(file_path, 'r') as f:
        return f.read()


def save_binary_file(file_name, data):
    f = open(file_name, "wb")
    f.write(data)
    f.close()
    print(f"File saved to: {file_name}")


def generate(prompt_file):
    # Read and parse prompt with frontmatter
    prompt_content = read_file_content(prompt_file)
    frontmatter, prompt_text = parse_frontmatter(prompt_content)
    
    # Extract frontmatter values
    title = frontmatter.get('title', 'generated_image')
    aspect_ratio = frontmatter.get('aspect_ratio', '1:1')
    resolution = frontmatter.get('resolution', '1K')
    temperature = float(frontmatter.get('temperature', '1.0'))
    batch = int(frontmatter.get('batch', '1'))
    
    # Read system instructions if specified in frontmatter
    system_instructions = None
    if 'instructions' in frontmatter:
        instructions_path = frontmatter['instructions']
        system_instructions = read_file_content(instructions_path)
    
    # Create artwork directory if it doesn't exist
    artwork_dir = "artwork"
    os.makedirs(artwork_dir, exist_ok=True)
    
    client = genai.Client(
        api_key=os.environ.get("GEMINI_API_KEY"),
    )

    model = "gemini-3-pro-image-preview"
    contents = [
        types.Content(
            role="user",
            parts=[
                types.Part.from_text(text=prompt_text),
            ],
        ),
    ]
    tools = [
        types.Tool(googleSearch=types.GoogleSearch(
        )),
    ]
    
    # Build config with optional system instructions
    config_params = {
        "temperature": temperature,
        "response_modalities": [
            "IMAGE",
            "TEXT",
        ],
        "image_config": types.ImageConfig(
            image_size=resolution,
            aspect_ratio=aspect_ratio,
        ),
        "tools": tools,
    }
    
    if system_instructions is not None:
        config_params["system_instruction"] = [
            types.Part.from_text(text=system_instructions),
        ]
    
    generate_content_config = types.GenerateContentConfig(**config_params)

    # Generate timestamp for unique filenames
    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")

    file_index = 1
    images_saved = 0

    if batch > 1:
        print(f"Generating {batch} images...")

    # Keep making API calls until we have the desired number of images
    while images_saved < batch:
        for chunk in client.models.generate_content_stream(
            model=model,
            contents=contents,
            config=generate_content_config,
        ):
            if (
                chunk.candidates is None
                or chunk.candidates[0].content is None
                or chunk.candidates[0].content.parts is None
            ):
                continue
            if chunk.candidates[0].content.parts[0].inline_data and chunk.candidates[0].content.parts[0].inline_data.data:
                # Stop if we've reached the desired batch count
                if images_saved >= batch:
                    break

                file_name = f"{title}_{timestamp}_{file_index}"
                file_index += 1
                inline_data = chunk.candidates[0].content.parts[0].inline_data
                data_buffer = inline_data.data
                file_extension = mimetypes.guess_extension(inline_data.mime_type)
                file_path = os.path.join(artwork_dir, f"{file_name}{file_extension}")
                save_binary_file(file_path, data_buffer)
                images_saved += 1

        # If we've saved enough images, exit the while loop
        if images_saved >= batch:
            break

    # Exit with error if no image was generated
    if images_saved == 0:
        print("Error: No image was generated.", file=sys.stderr)
        sys.exit(1)

    if batch > 1:
        print(f"\nCompleted: {images_saved} image(s) generated.")

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python create.py <prompt_file>")
        print("Example: python create.py prompt.md")
        sys.exit(1)
    
    prompt_file = sys.argv[1]
    if not os.path.exists(prompt_file):
        print(f"Error: Prompt file '{prompt_file}' not found.")
        sys.exit(1)
    
    generate(prompt_file)
