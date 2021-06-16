import json
import configparser
from json import decoder
import requests

def deflatten(data):
    deflattened = {}

    for key, value in data.items():
        parts = key.split('.')
        temp = deflattened
        for idx, part in enumerate(parts):
            if part not in temp:
                if idx == (len(parts) - 1):
                    temp[part] = value
                else:
                    temp[part] = {}
            temp = temp[part]

    return deflattened

config = configparser.ConfigParser()
config.read('config')
main = config['main']

keyQuery = '?key=' + main['key']
headers = {'accept': 'application/json'}

response = requests.get(main['host'] + '/api/v2/brands/' + main['brand'] + '/languages' + keyQuery, headers=headers)
if response.status_code != 200:
    raise Exception('Could not fetch brand languages')

languages = json.loads(response.text)

for language in languages:
    print(language['code'])
    response = requests.get(main['host'] + '/api/v2/brands/' + main['brand'] + '/languages/' + language['code'] + '/dictionary' + keyQuery + '&unescaped_unicode=1')
    if response.status_code != 200:
        print('COULD NOT FETCH TRANSLATIONS FOR LANGUAGE: %s' % language['code'])
        continue

    decoded = json.loads(response.text)
    parsed = deflatten(decoded)
    file = '../frontend/src/i18n/%s.json' % language['code']
    fd = open(file, 'w')
    fd.write(json.dumps(parsed, indent=2, ensure_ascii=False))
    fd.close()
