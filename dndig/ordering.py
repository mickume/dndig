"""Dependency ordering for prompt file processing."""

import logging
import os
from typing import List

from .config import parse_frontmatter

logger = logging.getLogger(__name__)


class CyclicDependencyError(Exception):
    """Raised when prompt files have circular dependencies."""


def topological_sort(files: List[str]) -> List[str]:
    """Sort prompt files so dependencies are processed first.

    Builds a dependency graph from frontmatter references and titles,
    then returns files in topological order. Files with no dependencies
    maintain their original order.

    Raises:
        CyclicDependencyError: If files have circular dependencies.
    """
    if len(files) <= 1:
        return list(files)

    file_info = {}
    title_to_file = {}

    for filepath in files:
        try:
            with open(filepath, 'r', encoding='utf-8') as f:
                content = f.read()
            frontmatter, _ = parse_frontmatter(content)
            title = frontmatter.get('title', '')
            references = frontmatter.get('references', []) or []
            file_info[filepath] = (title, references)
            if title:
                title_to_file[title] = filepath
        except Exception:
            file_info[filepath] = ('', [])

    edges = {f: [] for f in files}
    in_degree = {f: 0 for f in files}

    for filepath, (title, references) in file_info.items():
        for ref in references:
            stem = os.path.splitext(os.path.basename(ref))[0]
            dep_file = title_to_file.get(stem)
            if dep_file and dep_file != filepath:
                edges[dep_file].append(filepath)
                in_degree[filepath] += 1

    original_index = {f: i for i, f in enumerate(files)}
    queue = sorted(
        [f for f in files if in_degree[f] == 0],
        key=lambda f: original_index[f],
    )
    result = []

    while queue:
        node = queue.pop(0)
        result.append(node)
        for dependent in edges[node]:
            in_degree[dependent] -= 1
            if in_degree[dependent] == 0:
                queue.append(dependent)
                queue.sort(key=lambda f: original_index[f])

    if len(result) != len(files):
        cyclic = [f for f in files if f not in set(result)]
        details = []
        for f in cyclic:
            title, refs = file_info[f]
            matched = [
                os.path.splitext(os.path.basename(r))[0]
                for r in refs
                if title_to_file.get(os.path.splitext(os.path.basename(r))[0]) not in (None, f)
            ]
            details.append(f"  {f} (title: {title}, depends on: {', '.join(matched)})")
        raise CyclicDependencyError(
            "Circular dependency detected among prompt files:\n"
            + "\n".join(details)
        )

    return result
