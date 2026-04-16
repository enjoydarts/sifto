import unittest

from app.routers.summarize import SummarizeResponse


class SummarizeRouterTests(unittest.TestCase):
    def test_response_model_includes_genre_and_other_label(self):
        response = SummarizeResponse(
            summary="要約です。",
            topics=["AI"],
            genre="research",
            other_label="",
            translated_title="",
            score=0.72,
            score_breakdown={"importance": 0.8, "novelty": 0.5, "actionability": 0.6, "reliability": 0.9, "relevance": 0.7},
            score_reason="理由です。",
            score_policy_version="v4",
            llm={"provider": "test", "model": "test"},
        )

        dumped = response.model_dump() if hasattr(response, "model_dump") else response.dict()
        self.assertEqual(dumped["genre"], "research")
        self.assertEqual(dumped["other_label"], "")


if __name__ == "__main__":
    unittest.main()
