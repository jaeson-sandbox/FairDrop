"""Pin the Receive Again inventory and its shared Windows-safe runner."""
import importlib.util
from pathlib import Path
import unittest

SCRIPT = Path(__file__).with_name('verify-receive-again-mutations.py')
SPEC = importlib.util.spec_from_file_location('receive_again_mutations', SCRIPT)
assert SPEC is not None and SPEC.loader is not None
runner = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(runner)


class ReceiveAgainInventoryTest(unittest.TestCase):
    def test_complete_inventory_is_delegated_to_the_tested_runner(self):
        self.assertIs(runner.core.CASES, runner.CASES)
        self.assertEqual([case[0] for case in runner.CASES], [
            'admission', 'actual-reset', 'reset-bound', 'original-parent', 'waiting-state-guard',
            'incomplete-eligibility', 'cancelled-eligibility', 'text-eligibility', 'send-eligibility',
            'user-abandonment-clears', 'stale-work-fence', 'replacement-owner-finalizer', 'show-done-lock',
            'app-show-lock', 'app-repeat-entry', 'app-outcome-busy', 'show-focus-capture', 'show-focus-restore',
            'retirement-error-purpose'])
        for label, filename, test, before, after, name in runner.CASES:
            with self.subTest(case=label):
                source = (runner.core.ROOT / 'frontend' / filename).read_text(encoding='utf-8').replace('\r\n', '\n')
                self.assertEqual(source.count(before), 1)
                self.assertNotEqual(before, after)
                test_source = (runner.core.ROOT / 'frontend' / test).read_text(encoding='utf-8')
                if 'held incomplete' in name or 'held cancelled' in name:
                    self.assertIn('never enables Receive Again on a held %s outcome', test_source)
                else:
                    self.assertIn(name, test_source)


if __name__ == '__main__':
    unittest.main()
