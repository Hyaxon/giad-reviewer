"""GIAD example entrypoint for the mirrored official general reviewer."""

from giad_agents.code_review import main
from giad_agents.peer import run


if __name__ == "__main__":
    run(main, "code-review")
