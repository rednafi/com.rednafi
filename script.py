import httpx
import os
import re
import logging
import asyncio
from pathlib import Path
from tqdm.asyncio import tqdm_asyncio
from tqdm import tqdm
from httpx import HTTPStatusError, RequestError, TimeoutException

# Set up logging configuration
logging.basicConfig(
    filename="download_log.log",
    level=logging.INFO,
    format="%(asctime)s - %(levelname)s - %(message)s",
)

# Set up the base directory for markdown files and images
BASE_DIR = "content"
IMAGE_DIR = "static/images"

# Regex to match inline image references (e.g., ![text][ref])
INLINE_IMAGE_PATTERN = r"!\[.*?\]\[(.*?)\]"
# Regex to match image reference URLs at the bottom (e.g., [ref]: https://...)
REFERENCE_IMAGE_PATTERN = r"\[(.*?)\]:\s*(http.*?)\s*$"


# Create the directory if it doesn't exist
def ensure_directory_exists(directory):
    Path(directory).mkdir(parents=True, exist_ok=True)


# Retry mechanism for transient errors
async def retry_request(client, image_url, retries=3, backoff_factor=2.0):
    delay = 1
    for attempt in range(retries):
        try:
            logging.info(f"Attempting to download: {image_url}")
            response = await client.get(image_url)
            response.raise_for_status()  # Ensure we fail on bad status codes
            if response.history:
                logging.info(f"Redirection detected: {image_url} -> {response.url}")
            return response
        except (TimeoutException, RequestError) as exc:
            logging.warning(
                f"Error on attempt {attempt + 1} for {image_url}: {exc}. Retrying in {delay} seconds..."
            )
            await asyncio.sleep(delay)
            delay *= backoff_factor
        except HTTPStatusError as exc:
            logging.error(
                f"HTTP error {exc.response.status_code} for {image_url}: {exc}"
            )
            return None
    logging.error(f"Failed to download {image_url} after {retries} retries")
    return None


# Download image and save it in the right location
async def download_image(client, image_url, dest_path):
    response = await retry_request(client, image_url)
    if response:
        with open(dest_path, "wb") as f:
            f.write(response.content)
        logging.info(f"Successfully downloaded: {image_url}")
    else:
        logging.error(f"Failed to download image: {image_url}")


# Process a markdown file, find image URLs, and download them
async def process_markdown_file(client, file_path, file_progress):
    with open(file_path, "r", encoding="utf-8") as f:
        content = f.read()

    # Step 1: Find all image references at the bottom of the file
    image_references = dict(re.findall(REFERENCE_IMAGE_PATTERN, content))

    # Step 2: Find all inline image markers that refer to those references
    image_urls = []
    inline_references = re.findall(INLINE_IMAGE_PATTERN, content)
    for ref in inline_references:
        if ref in image_references:
            image_urls.append(
                (ref, image_references[ref])
            )  # Store reference name and URL

    if not image_urls:
        return

    # Create directory based on markdown file name
    markdown_filename = Path(file_path).stem
    image_subdir = os.path.join(IMAGE_DIR, markdown_filename)
    ensure_directory_exists(image_subdir)

    # Download all images with a progress bar
    for ref_name, image_url in tqdm(
        image_urls,
        desc=f"Downloading images from {file_path}",
        leave=False,
        position=file_progress,
    ):
        # Use the reference name for the image file and force a `.png` extension
        img_name = f"{ref_name}.png"
        img_path = os.path.join(image_subdir, img_name)
        await download_image(client, image_url, img_path)


# Walk through all markdown files and process them
async def main():
    markdown_files = []
    for root, _, files in os.walk(BASE_DIR):
        for file in files:
            if file.endswith(".md"):
                file_path = os.path.join(root, file)
                markdown_files.append(file_path)

    # Show a progress bar for markdown file processing
    async with httpx.AsyncClient(
        timeout=httpx.Timeout(30.0), follow_redirects=True
    ) as client:
        tasks = []
        with tqdm(
            total=len(markdown_files), desc="Processing markdown files"
        ) as file_bar:
            for idx, file_path in enumerate(markdown_files):
                tasks.append(process_markdown_file(client, file_path, idx))
                file_bar.update(1)
            await tqdm_asyncio.gather(*tasks)


if __name__ == "__main__":
    asyncio.run(main())
