from __future__ import annotations

from .models import ClassificationRequest, ClassificationResponse


def classify_stack(request: ClassificationRequest) -> ClassificationResponse:
    for sample in request.files:
        path = sample.get("path", "").lower()
        content = sample.get("content", "").lower()

        if path.endswith("package.json") or "\"next\"" in content:
            return ClassificationResponse(
                runtime="node",
                framework="nextjs",
                port=3000,
                build_command="npm run build",
                test_command="npm test",
                confidence=0.88,
            )
        if path.endswith("requirements.txt") or "fastapi" in content:
            return ClassificationResponse(
                runtime="python",
                framework="fastapi",
                port=8000,
                build_command="uvicorn app.main:app",
                test_command="pytest",
                confidence=0.84,
            )

    return ClassificationResponse(confidence=0.0)