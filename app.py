# Many thanks to: https://wikitech.wikimedia.org/wiki/Help:Toolforge/My_first_Flask_OAuth_tool

import os
import yaml

from flask import Flask, render_template, request, jsonify
from flask_cors import CORS
from mwviews.api import PageviewsClient
import numpy as np

app = Flask(__name__)

WIKIPEDIA_LANGUAGE_CODES = ['aa', 'ab', 'ace', 'ady', 'af', 'ak', 'als', 'am', 'an', 'ang', 'ar', 'arc', 'ary', 'arz', 'as', 'ast', 'atj', 'av', 'avk', 'awa', 'ay', 'az', 'azb', 'ba', 'ban', 'bar', 'bat-smg', 'bcl', 'be', 'be-x-old', 'bg', 'bh', 'bi', 'bjn', 'bm', 'bn', 'bo', 'bpy', 'br', 'bs', 'bug', 'bxr', 'ca', 'cbk-zam', 'cdo', 'ce', 'ceb', 'ch', 'cho', 'chr', 'chy', 'ckb', 'co', 'cr', 'crh', 'cs', 'csb', 'cu', 'cv', 'cy', 'da', 'de', 'din', 'diq', 'dsb', 'dty', 'dv', 'dz', 'ee', 'el', 'eml', 'en', 'eo', 'es', 'et', 'eu', 'ext', 'fa', 'ff', 'fi', 'fiu-vro', 'fj', 'fo', 'fr', 'frp', 'frr', 'fur', 'fy', 'ga', 'gag', 'gan', 'gcr', 'gd', 'gl', 'glk', 'gn', 'gom', 'gor', 'got', 'gu', 'gv', 'ha', 'hak', 'haw', 'he', 'hi', 'hif', 'ho', 'hr', 'hsb', 'ht', 'hu', 'hy', 'hyw', 'hz', 'ia', 'id', 'ie', 'ig', 'ii', 'ik', 'ilo', 'inh', 'io', 'is', 'it', 'iu', 'ja', 'jam', 'jbo', 'jv', 'ka', 'kaa', 'kab', 'kbd', 'kbp', 'kg', 'ki', 'kj', 'kk', 'kl', 'km', 'kn', 'ko', 'koi', 'kr', 'krc', 'ks', 'ksh', 'ku', 'kv', 'kw', 'ky', 'la', 'lad', 'lb', 'lbe', 'lez', 'lfn', 'lg', 'li', 'lij', 'lld', 'lmo', 'ln', 'lo', 'lrc', 'lt', 'ltg', 'lv', 'mai', 'map-bms', 'mdf', 'mg', 'mh', 'mhr', 'mi', 'min', 'mk', 'ml', 'mn', 'mnw', 'mr', 'mrj', 'ms', 'mt', 'mus', 'mwl', 'my', 'myv', 'mzn', 'na', 'nah', 'nap', 'nds', 'nds-nl', 'ne', 'new', 'ng', 'nl', 'nn', 'no', 'nov', 'nqo', 'nrm', 'nso', 'nv', 'ny', 'oc', 'olo', 'om', 'or', 'os', 'pa', 'pag', 'pam', 'pap', 'pcd', 'pdc', 'pfl', 'pi', 'pih', 'pl', 'pms', 'pnb', 'pnt', 'ps', 'pt', 'qu', 'rm', 'rmy', 'rn', 'ro', 'roa-rup', 'roa-tara', 'ru', 'rue', 'rw', 'sa', 'sah', 'sat', 'sc', 'scn', 'sco', 'sd', 'se', 'sg', 'sh', 'shn', 'si', 'simple', 'sk', 'sl', 'sm', 'smn', 'sn', 'so', 'sq', 'sr', 'srn', 'ss', 'st', 'stq', 'su', 'sv', 'sw', 'szl', 'szy', 'ta', 'tcy', 'te', 'tet', 'tg', 'th', 'ti', 'tk', 'tl', 'tn', 'to', 'tpi', 'tr', 'ts', 'tt', 'tum', 'tw', 'ty', 'tyv', 'udm', 'ug', 'uk', 'ur', 'uz', 've', 'vec', 'vep', 'vi', 'vls', 'vo', 'wa', 'war', 'wo', 'wuu', 'xal', 'xh', 'xmf', 'yi', 'yo', 'za', 'zea', 'zh', 'zh-classical', 'zh-min-nan', 'zh-yue', 'zu']

__dir__ = os.path.dirname(__file__)
app.config.update(
    yaml.safe_load(open(os.path.join(__dir__, 'default_config.yaml'))))
try:
    app.config.update(
        yaml.safe_load(open(os.path.join(__dir__, 'config.yaml'))))
except IOError:
    # It is ok if there is no local config file
    pass

# Enable CORS for API endpoints
#CORS(app, resources={'*': {'origins': '*'}})
CORS(app)

@app.route('/')
def index():
    mincount, lang, eps, sensitivity = validate_api_args()
    return render_template('index.html', mincount=mincount, lang=lang, eps=eps, sensitivity=sensitivity)

@app.route('/api/v1/pageviews')
def pageviews():
    mincount, lang, eps, sensitivity = validate_api_args()
    results = get_groundtruth(lang, mincount)
    add_laplace(results, eps, sensitivity, mincount)
    return jsonify({'params':{'mincount':mincount, 'lang':lang, 'eps':eps, 'sensitivity':sensitivity},
                    'results':results})

def get_groundtruth(lang, mincount):
    p = PageviewsClient(user_agent="isaac@wikimedia.org -- diff private toolforge")
    groundtruth = p.top_articles(project='{0}.wikipedia'.format(lang),
                                 access='all-access',
                                 year=None, month=None, day=None,  # defaults to yesterday
                                 limit=50)
    return {r['article']:{'gt-rank':r['rank'], 'gt-views':r['views']} for r in groundtruth if r['views'] >= mincount}

# Thanks to: https://github.com/Billy1900/Awesome-Differential-Privacy/blob/master/Laplace%26Exponetial/src/laplace_mechanism.py
def add_laplace(groundtruth, eps, sensitivity, mincount):
    dp_results = {}
    for title in groundtruth:
        views = groundtruth[title]['gt-views']
        dpviews = max(mincount, round(views + np.random.laplace(0, sensitivity / eps)))
        dp_results[title] = dpviews
    for dp_rank, title in enumerate(sorted(dp_results, key=dp_results.get, reverse=True), start=1):
        groundtruth[title]['dp-views'] = dp_results[title]
        groundtruth[title]['dp-rank'] = dp_rank

def validate_lang(lang):
    return lang in WIKIPEDIA_LANGUAGE_CODES

def validate_eps(eps):
    return eps <= 1 and eps > 0

def validate_sensitivity(sensitivity):
    return sensitivity >= 1

def validate_mincount(mincount):
    return mincount >= 0

def validate_api_args():
    lang = 'en'
    if 'lang' in request.args:
        if validate_lang(request.args['lang'].lower()):
            lang = request.args['lang'].lower()

    mincount = 1
    if 'mincount' in request.args:
        try:
            if validate_mincount(int(request.args['mincount'])):
                mincount = int(request.args['mincount'])
        except ValueError:
            pass

    eps = 1
    if 'eps' in request.args:
        try:
            if validate_eps(float(request.args['eps'])):
                eps = float(request.args['eps'])
        except ValueError:
            pass

    sensitivity = 1
    if 'sensitivity' in request.args:
        try:
            if validate_sensitivity(int(request.args['sensitivity'])):
                sensitivity = int(request.args['sensitivity'])
        except ValueError:
            pass

    return mincount, lang, eps, sensitivity