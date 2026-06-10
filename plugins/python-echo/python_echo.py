#!/usr/bin/env python3
import json
import sys


def main() -> int:
    request = json.load(sys.stdin)
    data = request.get("data", {})
    message = data.get("message") or "hello"
    json.dump(
        {
            "ok": True,
            "data": {
                "message": f"{message} from python plugin",
            },
        },
        sys.stdout,
    )
    sys.stdout.write("\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

