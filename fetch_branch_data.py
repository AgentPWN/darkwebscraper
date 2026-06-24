import json
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen


BASE_LINK = "https://devapi.fazaa.ae/api/member/getMemberDetails/CorrectData/"
BEARER_TOKEN = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJGYXphYSBTeXN0ZW0iLCJqdGkiOiJmOGQ0MzE2ZS03ZmViLTQ2ODktOTQ5ZC04MjJmMzlmYWIwZTEiLCJpYXQiOjE3ODExNTU0MTcsImV4cCI6MTc4MTE1NzIxN30._H9PlSLZsflQen198gUMLmbapeGt8ICIsQ84fdENfFM"


def fetch_url(url: str):
    request = Request(
        url,
        headers={
            "User-Agent": "Mozilla/5.0",
            "Authorization": f"Bearer {BEARER_TOKEN}",
        },
    )
    with urlopen(request, timeout=30) as response:
        content_type = response.headers.get("Content-Type", "")
        body = response.read().decode("utf-8", errors="replace")

        if "application/json" in content_type:
            return json.loads(body)

        try:
            return json.loads(body)
        except json.JSONDecodeError:
            return body


def main():
    for i in range(1000):
        url = f"{BASE_LINK}{i}"
        try:
            data = fetch_url(url)
            print(f"URL: {url}")
            print(data)
            print("-" * 80)
        except HTTPError as error:
            print(f"URL: {url}")
            print(f"HTTP error: {error.code} {error.reason}")
            print("-" * 80)
        except URLError as error:
            print(f"URL: {url}")
            print(f"Request failed: {error.reason}")
            print("-" * 80)


if __name__ == "__main__":
    main()