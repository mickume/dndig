"""Setup configuration for dndig."""

from setuptools import setup, find_packages
from pathlib import Path

# Read long description from README
readme_file = Path(__file__).parent / "README.md"
long_description = readme_file.read_text(encoding='utf-8') if readme_file.exists() else ''

setup(
    name='dndig',
    version='2.0.0',
    author='dndig Contributors',
    author_email='',
    description='AI image generation CLI using Google Gemini API',
    long_description=long_description,
    long_description_content_type='text/markdown',
    url='https://github.com/yourusername/dndig',
    packages=find_packages(exclude=['tests', 'tests.*']),
    classifiers=[
        'Development Status :: 4 - Beta',
        'Intended Audience :: Developers',
        'Topic :: Multimedia :: Graphics',
        'License :: OSI Approved :: MIT License',
        'Programming Language :: Python :: 3',
        'Programming Language :: Python :: 3.8',
        'Programming Language :: Python :: 3.9',
        'Programming Language :: Python :: 3.10',
        'Programming Language :: Python :: 3.11',
        'Programming Language :: Python :: 3.12',
        'Programming Language :: Python :: 3.13',
    ],
    python_requires='>=3.8',
    install_requires=[
        'google-genai>=0.1.0',
        'tqdm>=4.65.0',
    ],
    extras_require={
        'dev': [
            'pytest>=7.0.0',
            'pytest-cov>=4.0.0',
            'black>=23.0.0',
            'flake8>=6.0.0',
            'mypy>=1.0.0',
        ],
    },
    entry_points={
        'console_scripts': [
            'dndig=dndig.cli:main',
        ],
    },
    include_package_data=True,
    keywords='ai image-generation gemini cli',
    project_urls={
        'Bug Reports': 'https://github.com/yourusername/dndig/issues',
        'Source': 'https://github.com/yourusername/dndig',
    },
)
