import time
from requests_tor import RequestsTor
import re
from bs4 import BeautifulSoup
import urllib3

urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)

request = RequestsTor(tor_ports=(9050,), tor_cport=9051, autochange_id=False)

cookies = {
    "[REDACTED]"
}

headers = {
    'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8',
    'Accept-Encoding': 'gzip, deflate, br, zstd',
    'Accept-Language': 'en-US,en;q=0.5',
    'Connection': 'keep-alive',
    'Cookie': '[REDACTED]',
    'Host': 'breachedmw4otc2lhx7nqe4wyxfhpvy32ooz26opvqkmmrbg73c7ooad.onion',
    'Sec-Fetch-Dest': 'document',
    'Sec-Fetch-Mode': 'navigate',
    'Sec-Fetch-Site': 'none',
    'Sec-Fetch-User': '?1',
    'Sec-GPC': '1',
    'Upgrade-Insecure-Requests': '1',
    'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0'
}

with open('names.txt', 'r') as f:
    names = [line.strip() for line in f]

base_url = "http://breachedmw4otc2lhx7nqe4wyxfhpvy32ooz26opvqkmmrbg73c7ooad.onion/search.php?action=do_search&keywords={}&postthread=1&author=&matchusername=1&forums[]=all&findthreadst=1&numreplies=&postdate=0&pddir=1&threadprefix[]=any&sortby=lastpost&sortordr=desc&showresults=threads&submit=Search"

for name in names:
    url = base_url.format(name)
    r = request.get(url, cookies=cookies, headers=headers, verify=False, allow_redirects=False)
    if 300 <= r.status_code < 400:
        redirect_url = r.headers.get('Location')
        r = request.get(redirect_url, cookies=cookies, headers=headers, verify=False, allow_redirects=False)
    if 300 <= r.status_code < 400:
        continue
    if "Sorry, but no results were returned using the query information you provided. Please redefine your search terms and try again." in r.text:
        print(f"No results for: {name}")
        time.sleep(5)
        continue
    soup = BeautifulSoup(r.text, 'html.parser')
    thread_links = []
    for a_tag in soup.find_all('a', href=True):
        if a_tag['href'].startswith('Thread-') and f'highlight={name}' in a_tag['href']:
            thread_links.append(a_tag['href'])
    for thread_url in thread_links:
        thread_url = f"http://breachedmw4otc2lhx7nqe4wyxfhpvy32ooz26opvqkmmrbg73c7ooad.onion//{thread_url}"
        thread_response = request.get(thread_url, cookies=cookies, headers=headers, verify=False, allow_redirects=False)
        if 300 <= thread_response.status_code < 400:
            redirect_url = thread_response.headers.get('Location')
            thread_response = request.get(redirect_url, cookies=cookies, headers=headers, verify=False, allow_redirects=False)
        thread_soup = BeautifulSoup(thread_response.text, 'html.parser')
        code_tags = thread_soup.find_all('code')
        name_found = False
        for code_tag in code_tags:
            if name in code_tag.get_text():
                name_found = True
                break
        if name_found:
            print(f"Name '{name}' found in thread: {thread_url}")
        else:
            print(f"Name '{name}' not found in thread: {thread_url}")
    time.sleep(5)
