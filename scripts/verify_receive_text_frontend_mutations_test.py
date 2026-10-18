"""Verdict parsing for colored and uncolored Vitest mutation output."""

import importlib.util
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest import mock


SCRIPT = Path(__file__).with_name('verify-receive-text-frontend-mutations.py')
SPEC = importlib.util.spec_from_file_location('receive_text_frontend_mutations', SCRIPT)
assert SPEC is not None and SPEC.loader is not None
runner = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(runner)

NAME = 'fences old copy across Done and a new receipt while allowing the new copy'
PLAIN = (f' FAIL src/transfer/receiveText.test.tsx > inbound text metadata > {NAME}\n'
         'AssertionError: expected false to be true\n'
         'Test Files  1 failed (1)\nTests  1 failed | 14 skipped (15)\n')
COLORED = ('\x1b[41m FAIL \x1b[0m src/transfer/receiveText.test.tsx\x1b[2m > '
           '\x1b[0minbound text metadata\x1b[2m > \x1b[0m' + NAME + '\n'
           '\x1b[31m\x1b[1mAssertionError\x1b[0m: expected false to be true\n'
           '\x1b[2mTest Files \x1b[0m \x1b[31m1 failed\x1b[0m (1)\n'
           '\x1b[2mTests \x1b[0m \x1b[31m1 failed\x1b[0m (1)\n')


class VerdictTest(unittest.TestCase):
    def test_named_assertion_accepts_plain_and_colored_output(self):
        for output in (PLAIN, COLORED):
            with self.subTest(colored=output is COLORED):
                runner.verify_result(1, output, NAME, 'mutation.log', True)

    def test_baseline_accepts_plain_and_colored_output(self):
        for output in ('Test Files  1 passed (1)\nTests  1 passed (15)\n',
                       '\x1b[2mTest Files \x1b[0m \x1b[32m1 passed\x1b[0m (1)\n'
                       '\x1b[2mTests \x1b[0m \x1b[32m1 passed\x1b[0m (15)\n'):
            runner.verify_result(0, output, NAME, 'baseline.log', False)

    def test_non_target_failure_is_not_a_kill(self):
        other = PLAIN.replace(f'> {NAME}\n', '> unrelated test\n') + NAME + '\n'
        with self.assertRaisesRegex(RuntimeError, 'named assertion'):
            runner.verify_result(1, other, NAME, 'mutation.log', True)

    def test_compiler_failure_is_not_a_kill_even_with_named_assertion(self):
        for diagnostic in ('Error: Transform failed', "Cannot find module '/@fs/C:/Users/runneradmin/...'"):
            with self.subTest(diagnostic=diagnostic):
                with self.assertRaisesRegex(RuntimeError, 'compiler/import'):
                    runner.verify_result(1, COLORED + diagnostic + '\n', NAME, 'mutation.log', True)

    def test_import_failure_from_windows_artifact_is_not_a_baseline(self):
        output = ("FAIL src/transfer/receiveText.test.tsx [ src/transfer/receiveText.test.tsx ]\n"
                  "Error: Cannot find module '/@fs/C:/Users/runneradmin/AppData/Local/Temp/frontend/src/transfer/receiveText.test.tsx'\n"
                  'Test Files  1 failed (1)\nTests  no tests\n')
        with self.assertRaisesRegex(RuntimeError, 'baseline did not pass'):
            runner.verify_result(1, output, NAME, 'baseline.log', False)

    def test_failed_status_or_missing_assertion_is_not_a_kill(self):
        for status, output in ((0, PLAIN), (1, PLAIN.replace('AssertionError:', 'Error:'))):
            with self.assertRaisesRegex(RuntimeError, 'named assertion'):
                runner.verify_result(status, output, NAME, 'mutation.log', True)

    def test_dependency_fallback_copies_when_symlink_is_refused(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            frontend = root / 'frontend'
            source = root / 'installed'
            frontend.mkdir()
            source.mkdir()
            (source / 'vitest.mjs').write_text('installed', encoding='utf-8')
            with mock.patch.object(Path, 'symlink_to', side_effect=OSError('privilege denied')):
                runner.prepare_dependencies(frontend, source)
            self.assertEqual((frontend / 'node_modules/vitest.mjs').read_text(encoding='utf-8'), 'installed')

    def test_process_launch_error_is_saved(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            log = root / 'baseline.log'
            with mock.patch.object(runner.subprocess, 'run', side_effect=OSError('node unavailable')):
                with self.assertRaisesRegex(OSError, 'node unavailable'):
                    runner.run(root, 'test.tsx', NAME, log, False)
            self.assertIn('OSError: node unavailable', log.read_text(encoding='utf-8'))

    def test_timeout_preserves_partial_output(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            log = root / 'mutation.log'
            failure = runner.subprocess.TimeoutExpired(['node'], 120, output=b'partial failure\n')
            with mock.patch.object(runner.subprocess, 'run', side_effect=failure):
                with self.assertRaises(runner.subprocess.TimeoutExpired):
                    runner.run(root, 'test.tsx', NAME, log, True)
            self.assertIn('partial failure\n', log.read_text(encoding='utf-8'))

    def test_subprocess_decodes_vitest_output_as_utf8(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            log = root / 'baseline.log'
            completed = SimpleNamespace(returncode=0, stdout='Test Files  1 passed\nTests  1 passed\n')
            with mock.patch.object(runner.subprocess, 'run', return_value=completed) as process:
                runner.run(root, 'test.tsx', NAME, log, False)
            self.assertEqual(process.call_args.kwargs['encoding'], 'utf-8')
            self.assertEqual(process.call_args.kwargs['errors'], 'replace')

    def test_workspace_and_vitest_cwd_share_the_resolved_repository_root(self):
        with runner.mutation_workspace('path-identity') as workspace:
            self.assertEqual(workspace.parent, runner.ROOT)
            self.assertEqual(workspace, workspace.resolve())
            frontend = workspace / 'frontend'
            frontend.mkdir()
            log = workspace / 'baseline.log'
            completed = SimpleNamespace(returncode=0, stdout='Test Files  1 passed\nTests  1 passed\n')
            with mock.patch.object(runner.subprocess, 'run', return_value=completed) as process:
                runner.run(frontend, 'src/transfer/receiveText.test.tsx', NAME, log, False)
            self.assertEqual(process.call_args.kwargs['cwd'], frontend)
            self.assertEqual(process.call_args.kwargs['cwd'].parent.parent, runner.ROOT)


if __name__ == '__main__':
    unittest.main()
