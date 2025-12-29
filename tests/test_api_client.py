"""Tests for api_client module."""

import os
import pytest
from unittest.mock import Mock, patch
from dndig.api_client import GeminiClient, GeminiAPIError
from dndig.config import GenerationConfig


class TestGeminiClient:
    """Tests for GeminiClient class."""

    def test_init_with_api_key(self):
        """Test initialization with explicit API key."""
        with patch('dndig.api_client.genai.Client'):
            client = GeminiClient(api_key='test_key')
            assert client.api_key == 'test_key'

    def test_init_from_environment(self):
        """Test initialization from environment variable."""
        with patch.dict(os.environ, {'GEMINI_API_KEY': 'env_key'}):
            with patch('dndig.api_client.genai.Client'):
                client = GeminiClient()
                assert client.api_key == 'env_key'

    def test_init_no_api_key_raises(self):
        """Test initialization without API key raises error."""
        with patch.dict(os.environ, {}, clear=True):
            with pytest.raises(GeminiAPIError, match="API key not found"):
                GeminiClient()

    def test_validate_connection(self):
        """Test connection validation."""
        with patch('dndig.api_client.genai.Client'):
            client = GeminiClient(api_key='test_key')
            assert client.validate_connection() is True

    @patch('dndig.api_client.genai.Client')
    def test_generate_image_stream(self, mock_client_class):
        """Test image generation streaming."""
        mock_client = Mock()
        mock_client_class.return_value = mock_client

        # Mock the stream response
        mock_chunk = Mock()
        mock_client.models.generate_content_stream.return_value = [mock_chunk]

        client = GeminiClient(api_key='test_key')
        config = GenerationConfig()

        result = list(client.generate_image_stream(
            prompt="test prompt",
            config=config,
        ))

        assert len(result) == 1
        assert result[0] == mock_chunk
        mock_client.models.generate_content_stream.assert_called_once()
