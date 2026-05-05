"""Tests for dependency ordering of prompt files."""

import os
import pytest
from dndig.ordering import topological_sort, CyclicDependencyError


def _write_prompt(path, title, references=None):
    refs = ""
    if references:
        items = ", ".join(references)
        refs = f"\nreferences: [{items}]"
    path.write_text(f"---\ntitle: {title}{refs}\n---\nPrompt text\n")
    return str(path)


class TestTopologicalSort:

    def test_empty_list(self):
        assert topological_sort([]) == []

    def test_single_file(self, tmp_path):
        f = _write_prompt(tmp_path / "a.md", "alpha")
        assert topological_sort([f]) == [f]

    def test_no_dependencies_preserves_order(self, tmp_path):
        a = _write_prompt(tmp_path / "a.md", "alpha")
        b = _write_prompt(tmp_path / "b.md", "beta")
        c = _write_prompt(tmp_path / "c.md", "gamma")
        result = topological_sort([a, b, c])
        assert result == [a, b, c]

    def test_simple_dependency_reorders(self, tmp_path):
        a = _write_prompt(tmp_path / "a_scene.md", "scene", references=["base.jpg"])
        b = _write_prompt(tmp_path / "b_base.md", "base")
        result = topological_sort([a, b])
        assert result == [b, a]

    def test_chain_dependency(self, tmp_path):
        a = _write_prompt(tmp_path / "a.md", "final", references=["middle.png"])
        b = _write_prompt(tmp_path / "b.md", "middle", references=["first.jpg"])
        c = _write_prompt(tmp_path / "c.md", "first")
        result = topological_sort([a, b, c])
        assert result == [c, b, a]

    def test_diamond_dependency(self, tmp_path):
        a = _write_prompt(tmp_path / "a_root.md", "root")
        b = _write_prompt(tmp_path / "b_left.md", "left", references=["root.jpg"])
        c = _write_prompt(tmp_path / "c_right.md", "right", references=["root.jpg"])
        d = _write_prompt(tmp_path / "d_final.md", "final", references=["left.jpg", "right.jpg"])
        result = topological_sort([a, b, c, d])
        assert result.index(a) < result.index(b)
        assert result.index(a) < result.index(c)
        assert result.index(b) < result.index(d)
        assert result.index(c) < result.index(d)

    def test_cycle_raises_error(self, tmp_path):
        a = _write_prompt(tmp_path / "a.md", "alpha", references=["beta.jpg"])
        b = _write_prompt(tmp_path / "b.md", "beta", references=["alpha.jpg"])
        with pytest.raises(CyclicDependencyError):
            topological_sort([a, b])

    def test_cycle_error_message_is_descriptive(self, tmp_path):
        a = _write_prompt(tmp_path / "a.md", "alpha", references=["beta.jpg"])
        b = _write_prompt(tmp_path / "b.md", "beta", references=["alpha.jpg"])
        with pytest.raises(CyclicDependencyError, match="Circular dependency"):
            topological_sort([a, b])

    def test_unmatched_references_ignored(self, tmp_path):
        a = _write_prompt(tmp_path / "a.md", "alpha", references=["landscape.jpg"])
        b = _write_prompt(tmp_path / "b.md", "beta")
        result = topological_sort([a, b])
        assert result == [a, b]

    def test_self_reference_no_loop(self, tmp_path):
        a = _write_prompt(tmp_path / "a.md", "castle", references=["castle.jpg"])
        result = topological_sort([a])
        assert result == [a]

    def test_reference_path_stem_matching(self, tmp_path):
        a = _write_prompt(tmp_path / "a.md", "scene", references=["../artwork/castle.jpg"])
        b = _write_prompt(tmp_path / "b.md", "castle")
        result = topological_sort([a, b])
        assert result == [b, a]

    def test_multiple_references_per_file(self, tmp_path):
        a = _write_prompt(tmp_path / "a.md", "composite", references=["bg.png", "char.jpg"])
        b = _write_prompt(tmp_path / "b.md", "bg")
        c = _write_prompt(tmp_path / "c.md", "char")
        result = topological_sort([a, b, c])
        assert result.index(b) < result.index(a)
        assert result.index(c) < result.index(a)

    def test_mixed_referenced_and_unreferenced(self, tmp_path):
        a = _write_prompt(tmp_path / "a.md", "alpha")
        b = _write_prompt(tmp_path / "b.md", "beta", references=["delta.jpg"])
        c = _write_prompt(tmp_path / "c.md", "gamma")
        d = _write_prompt(tmp_path / "d.md", "delta")
        result = topological_sort([a, b, c, d])
        assert result.index(d) < result.index(b)
        assert result.index(a) < result.index(b)

    def test_stable_order_among_independent_files(self, tmp_path):
        a = _write_prompt(tmp_path / "a.md", "alpha")
        b = _write_prompt(tmp_path / "b.md", "beta")
        c = _write_prompt(tmp_path / "c.md", "gamma", references=["alpha.jpg"])
        result = topological_sort([a, b, c])
        assert result == [a, b, c]

    def test_three_node_cycle(self, tmp_path):
        a = _write_prompt(tmp_path / "a.md", "alpha", references=["gamma.jpg"])
        b = _write_prompt(tmp_path / "b.md", "beta", references=["alpha.jpg"])
        c = _write_prompt(tmp_path / "c.md", "gamma", references=["beta.jpg"])
        with pytest.raises(CyclicDependencyError):
            topological_sort([a, b, c])
