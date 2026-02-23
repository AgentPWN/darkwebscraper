import time
from requests_tor import RequestsTor
import re
from bs4 import BeautifulSoup
import urllib3

urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)
URL_TEMPLATE = "https://dreadytofatroptsdj6io7l3xptbet6onoyno2yv7jicoxknyazubrad.onion/search/?q={name}"

HEADERS = {
    "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
    "Accept-Encoding": "gzip, deflate, br, zstd",
    "Accept-Language": "en-US,en;q=0.5",
    "Connection": "keep-alive",
    "Cookie": "[REDACTED]",
    "Host": "dreadytofatroptsdj6io7l3xptbet6onoyno2yv7jicoxknyazubrad.onion",
    "Priority": "u=0, i",
    "Sec-Fetch-Dest": "document",
    "Sec-Fetch-Mode": "navigate",
    "Sec-Fetch-Site": "none",
    "Sec-Fetch-User": "?1",
    "Sec-GPC": "1",
    "Upgrade-Insecure-Requests": "1",
    "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0"
}

rt = RequestsTor(
    tor_ports=(9050,),
    tor_cport=9051,
    password=None,
    autochange_id=False
)

with open("names.txt", "r") as file:
    names = [line.strip() for line in file if line.strip()]

for name in names:
    url = URL_TEMPLATE.format(name=name)
    try:
        response = rt.get(url, headers=HEADERS,verify=False)
        if response.status_code == 200:
            soup = BeautifulSoup(response.text, "html.parser")
            if "No results could be found for your search term" not in response.text:
                print(f"Data found for {name}:")
        else:
            print(f"Failed to fetch data for {name}. HTTP Status: {response.status_code}")
    except Exception as e:
        print(f"An error occurred for {name}: {e}")
    time.sleep(5)