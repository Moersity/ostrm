import importlib.util
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('release', Path(__file__).resolve().parents[2] / 'build/release.py')
r = importlib.util.module_from_spec(spec)
spec.loader.exec_module(r)

class Versions(unittest.TestCase):
    def test_semantics(self):
        for messages, labels, expected in [(['fix: repair'], [], 'patch'), (['feat(scan): movies'], [], 'minor'), (['feat!: change'], [], 'major'), (['fix: x\n\nBREAKING CHANGE: schema'], [], 'major'), (['feat: x'], ['semver:patch'], 'minor'), ([], ['semver:major'], 'major')]:
            with self.subTest(messages=messages):
                self.assertEqual(r.bump_kind(messages, labels), expected)
        self.assertEqual(r.increment((3, 9, 9), 'minor'), (3, 10, 0))
        self.assertEqual(r.increment((3, 9, 9), 'major'), (4, 0, 0))
        self.assertIsNone(r.version_tuple('v03.1.0'))
        self.assertIsNone(r.version_tuple('v3.1.0-beta.1'))

class Planning(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.cwd = os.getcwd()
        os.chdir(self.tmp.name)
        self.git('init', '-q')
        self.git('config', 'user.email', 'test@example.com')
        self.git('config', 'user.name', 'Test')
        self.commit('initial')
        self.git('tag', 'v3.0.0')
        self.releases = [dict(tag_name='v3.0.0', draft=False, prerelease=False)]
        self.commit('fix: update')
    def tearDown(self):
        os.chdir(self.cwd)
        self.tmp.cleanup()
    def git(self, *args):
        return subprocess.check_output(['git', *args], text=True).strip()
    def commit(self, message):
        self.git('commit', '--allow-empty', '-qm', message)
    def event(self, title='maintenance'):
        return {'pull_request': dict(merged=True, merge_commit_sha=self.git('rev-parse', 'HEAD'), title=title, base={'ref':'main','repo':{'full_name':'owner/repo'}}, head={'repo':{'full_name':'owner/repo'}})}
    def plan(self, title='maintenance'):
        return r.plan_release(self.event(title), self.releases, 'owner/repo')
    def test_patch_and_feature(self):
        self.assertEqual(self.plan()['version'], '3.0.1')
        self.assertEqual(self.plan('feat: movie support')['version'], '3.1.0')
    def test_full_history_and_highest_severity(self):
        self.commit('feat: movies')
        self.commit('fix: polish')
        self.assertEqual(self.plan()['version'], '3.1.0')
        self.commit('refactor!: incompatible schema')
        self.assertEqual(self.plan()['version'], '4.0.0')
    def test_published_retry(self):
        self.git('tag', 'v3.0.1')
        self.releases.append(dict(tag_name='v3.0.1', draft=False, prerelease=False))
        self.assertTrue(self.plan()['published'])
    def test_draft_retry(self):
        self.releases.append(dict(tag_name='v3.1.0', draft=True, prerelease=False, target_commitish=self.git('rev-parse','HEAD')))
        self.assertEqual(self.plan()['version'], '3.1.0')
    def test_collision(self):
        self.git('tag', 'v3.0.1', 'v3.0.0')
        with self.assertRaises(ValueError): self.plan()
    def test_old_commit(self):
        old = self.git('rev-parse', 'HEAD')
        self.commit('fix: newer')
        self.git('tag', 'v3.0.1')
        self.releases.append(dict(tag_name='v3.0.1', draft=False, prerelease=False))
        self.git('checkout', '-q', old)
        with self.assertRaises(subprocess.CalledProcessError): self.plan()
    def test_wrong_destination(self):
        event = self.event()
        event['pull_request']['base']['ref'] = 'beta'
        with self.assertRaises(ValueError): r.plan_release(event, self.releases, 'owner/repo')

class Publishing(unittest.TestCase):
    def test_create_upload_publish_by_id(self):
        import hashlib
        with tempfile.TemporaryDirectory() as tmp:
            cwd = os.getcwd()
            try:
                os.chdir(tmp)
                Path('dist').mkdir()
                Path('dist/a.zip').write_bytes(b'installer')
                Path('RELEASE-NOTES.md').write_text('Install instructions')
                digest = 'sha256:' + hashlib.sha256(b'installer').hexdigest()
                asset = dict(name='a.zip', digest=digest)
                responses = [dict(body='Changes'), dict(id=123, assets=[]), asset, dict(assets=[asset]), dict(draft=False)]
                with patch.object(r, 'command', return_value='a'*40), patch.object(r, 'gh_json', side_effect=responses) as api:
                    r.publish(dict(version='3.1.0', sha='a'*40, previous='v3.0.0'), [], 'owner/repo')
                    calls = [c.args for c in api.call_args_list]
                    self.assertIn('https://uploads.github.com/repos/owner/repo/releases/123/assets?name=a.zip', calls[2])
                    self.assertEqual(calls[3], ('api', 'repos/owner/repo/releases/123'))
                    self.assertIn('repos/owner/repo/releases/123', calls[4])
                    self.assertFalse(any('/tags/' in str(c) for c in calls))
            finally:
                os.chdir(cwd)

    def test_published_is_immutable(self):
        with patch.object(r, 'command', return_value='a'*40) as cmd:
            r.publish(dict(version='3.1.0', sha='a'*40), [dict(tag_name='v3.1.0', draft=False)], 'owner/repo')
            self.assertTrue(all(call.args[0] == 'git' for call in cmd.call_args_list))
    def test_wrong_draft_rejected(self):
        with patch.object(r, 'command', return_value='a'*40) as cmd:
            with self.assertRaises(ValueError):
                r.publish(dict(version='3.1.0', sha='a'*40), [dict(tag_name='v3.1.0', draft=True, target_commitish='b'*40)], 'owner/repo')
            self.assertEqual(cmd.call_count, 1)

if __name__ == '__main__': unittest.main()
