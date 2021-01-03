import math
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
CORS(app)

@app.route('/')
def index():
    """Load UI, optionally with form prefilled based on URL parameters. User will still need to hit `Submit` button."""
    mincount, lang, eps, sensitivity = validate_api_args()
    return render_template('index.html', mincount=mincount, lang=lang, eps=eps, sensitivity=sensitivity)

@app.route('/api/v1/pageviews')
def pageviews():
    """Compute differentially-private version of top pageviews for a Wikipedia language and compare to groundtruth."""
    _, lang, eps, sensitivity = validate_api_args()
    results = get_groundtruth(lang)
    add_laplace(results, eps, sensitivity)
    return jsonify({'params':{'lang':lang, 'eps':eps, 'sensitivity':sensitivity, 'qual-eps':qual_eps(eps)},
                    'results':results})

def get_groundtruth(lang):
    """Get actual counts of top articles and their pageviews from a Wikipedia from yesterday."""
    p = PageviewsClient(user_agent="isaac@wikimedia.org -- diff private toolforge")
    groundtruth = p.top_articles(project='{0}.wikipedia'.format(lang),
                                 access='all-access',
                                 year=None, month=None, day=None,  # defaults to yesterday
                                 limit=50)
    return {r['article']:{'gt-rank':r['rank'], 'gt-views':r['views']} for r in groundtruth}

# Thanks to: https://github.com/Billy1900/Awesome-Differential-Privacy/blob/master/Laplace%26Exponetial/src/laplace_mechanism.py
def add_laplace(groundtruth, eps, sensitivity, floor=0):
    """Add in differentially-private version of results."""
    dp_results = {}
    for title in groundtruth:
        views = groundtruth[title]['gt-views']
        dpviews = max(floor, round(views + np.random.laplace(0, sensitivity / eps)))
        dp_results[title] = dpviews
    for dp_rank, title in enumerate(sorted(dp_results, key=dp_results.get, reverse=True), start=1):
        groundtruth[title]['dp-views'] = dp_results[title]
        groundtruth[title]['dp-rank'] = dp_rank
        groundtruth[title]['do-aggregate'] = do_aggregate(groundtruth[title]['dp-views'], sensitivity, eps)

def qual_eps(eps, p=0.5):
    """Provide a qualitative explanation for what a particular epsilon value means.

    Recommended description:
        If someone believed a user was in the data with probability p,
        then at most after seeing the data they will be qual_eps(eps, p) certain
        (assuming the sensitivity value is correct).
        e.g., for eps=1; p=0.5, they'd go from 50% certainty to at most 73.1% certainty.

    Parameters:
        eps: epsilon value that quantifies "privacy" level of differential privacy.
        p: initial belief that a given user is in the data -- e.g.,:
            0.5 represents complete uncertainty (50/50 chance)
            0.01 represents high certainty the person isn't in the data
            0.99 represents high certainty the person is in the data
    """
    # unacceptable values; p must be (0,1) -- i.e. some uncertainty
    if p > 0 and p < 1:
        return ((math.e ** eps) * p) / (1 + (((math.e ** eps) - 1) * p))
    else:
        return None


def do_aggregate(noisedX, sensitivity, eps, alpha=0.5, prop_within=0.25):
    """Check whether noisy data X is at least (100 * alpha)% of being within Y% of true value.
    Doesn't use true value (only noisy value and parameters) so no privacy cost to this.
    Should identify in advance what threshold -- e.g., 50% probability within 25% of actual value -- in advance
    to determine whether to keep the data or suppress it until it can be further aggregated so more signal to noise.
    See a more complete description in the paper below for how to effectively use this data.

    Based on:
    * Description: https://arxiv.org/pdf/2009.01265.pdf#section.4
    * Code: https://github.com/google/differential-privacy/blob/main/java/main/com/google/privacy/differentialprivacy/LaplaceNoise.java#L127

    Parameters:
        noisedX: the count after adding Laplace noise
        sensitivity: L1 sensitivity for Laplace
        eps: selected epsilon value
        alpha: how confident (0.5 = 50%; 0.1 = 90%) should we be that the noisy data is within (100 * prop_within)% of the true data?
        prop_within: how close (0.25 = 25%) to actual value should we expect the noisy data to be?
    """
    # Divide alpha by 2 because two-tailed
    rank = alpha / 2
    lbda = sensitivity / eps
    # get confidence interval; this is symmetric where `lower bound = noisedX - ci` and `upper bound = noisedX + ci`
    ci = abs(lbda * np.log(2 * rank))
    if ci > (prop_within * noisedX):
        return 'Yes'
    else:
        return 'No'

def validate_lang(lang):
    return lang in WIKIPEDIA_LANGUAGE_CODES

def validate_eps(eps):
    return eps > 0 and np.isfinite(eps)

def validate_sensitivity(sensitivity):
    return sensitivity > 0 and np.isfinite(sensitivity)

def validate_mincount(mincount):
    return mincount >= 0

def validate_api_args():
    lang = 'en'
    if 'lang' in request.args:
        if validate_lang(request.args['lang'].lower()):
            lang = request.args['lang'].lower()

    mincount = 0
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