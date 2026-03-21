import unittest

from app.services.facts_check_common import detect_facts_check_coverage_issue


class DetectFactsCheckCoverageIssueTests(unittest.TestCase):
    def test_empty_facts_fails(self):
        result = detect_facts_check_coverage_issue(
            "これは本文です。重要な情報を含む長い記事です。",
            [],
        )
        self.assertIsNotNone(result)
        self.assertEqual(result.get("verdict"), "fail")
        self.assertIn("空", result.get("short_comment", ""))

    def test_short_facts_warns_when_content_is_long(self):
        facts = ["要約が短い"]
        result = detect_facts_check_coverage_issue(
            "A" * 4000,
            facts,
        )
        self.assertIsNotNone(result)
        self.assertIn(result.get("verdict"), {"fail", "warn"})

    def test_placeholder_facts_fail(self):
        result = detect_facts_check_coverage_issue(
            "これは本文です。",
            ["事実1", "事実2", "事実3"],
        )
        self.assertIsNotNone(result)
        self.assertEqual(result.get("verdict"), "fail")

    def test_normal_facts_passes(self):
        result = detect_facts_check_coverage_issue(
            "A" * 2400,
            [
                "主要な事実として新製品の発売が発表された。",
                "価格は月額980円の新プランを提供する。",
                "対象地域は国内外の主要都市に拡大した。",
                "過去の運用実績を踏まえた改善計画も同時に示した。",
                "来年の夏に追加機能を投入する予定だ。",
            ],
        )
        self.assertIsNone(result)

    def test_long_content_requires_more_facts(self):
        result = detect_facts_check_coverage_issue(
            "A" * 6000,
            [
                "新製品の発売が発表された。",
                "価格は月額980円の新プランを提供する。",
                "対象地域は主要都市に拡大した。",
                "来年の夏に追加機能を投入する予定。",
                "導入事例が継続的に増えている。",
            ],
        )
        self.assertIsNotNone(result)
        self.assertIn(result.get("verdict"), {"warn", "fail"})


if __name__ == "__main__":
    unittest.main()
