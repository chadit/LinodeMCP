"""The redaction the security-question preview owns.

Every rule this tool refuses an argument by is declared on its input message and
evaluated by ``linodemcp.tools.constraints``. What is left here is the rebuilt
body a dry run reports, since the answers are security material. The Go twin is
``profile_secrets_preview_test.go`` in ``go/internal/toolhooks``.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any

import pytest

from linodemcp import toolhooks

if TYPE_CHECKING:
    from linodemcp.config import Config

PROFILE_SECURITY_QUESTIONS_PATH = "/profile/security-questions"


@pytest.mark.parametrize(
    ("body", "expected"),
    [
        (
            {
                "security_questions": [
                    {"question_id": 1, "response": "first answer"},
                    {"question_id": 7, "response": "second answer"},
                ]
            },
            [
                {"question_id": 1, "response": "[redacted]"},
                {"question_id": 7, "response": "[redacted]"},
            ],
        ),
        ({"security_questions": ["not an object"]}, []),
        (
            {"security_questions": [{"response": "no id here"}]},
            [{"question_id": None, "response": "[redacted]"}],
        ),
        ({}, []),
        (None, []),
    ],
)
async def test_security_question_answer_preview_redacts_each_answer(
    sample_config: Config, body: dict[str, Any] | None, expected: list[dict[str, Any]]
) -> None:
    """The ids stay readable because they are what a caller checks before
    confirming; the answers do not, because they are the security material.

    The Go twin is
    TestLinodeProfileSecurityQuestionAnswerPreviewRedactsEachAnswer.
    """
    result = await toolhooks.linode_profile_security_question_answer_preview(
        sample_config,
        {"dry_run": True},
        "POST",
        PROFILE_SECURITY_QUESTIONS_PATH,
        body,
    )

    preview = json.loads(result[0].text)

    assert preview["would_execute"]["body"] == {"security_questions": expected}
    assert preview["side_effects"] == [
        "The profile's security question answers are saved."
    ]
