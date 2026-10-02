from nuxt import route, logger, Request
from nuxt.repositorys.validation import fields, use_args
import os
import traceback
import tiktoken

BASE_DIR = os.path.dirname(os.path.abspath(__file__))
TIKTOKEN_CACHE_DIR = os.environ.get("TIKTOKEN_CACHE_DIR", os.path.join(BASE_DIR, "tiktoken_cache"))
os.makedirs(TIKTOKEN_CACHE_DIR, exist_ok=True)
os.environ["TIKTOKEN_CACHE_DIR"] = TIKTOKEN_CACHE_DIR

encoding_cache = {}


@route("/tokenizer/<str:model_name>", methods=["POST"])
@use_args({
    "role": fields.Str(required=True),
    "content": fields.Str(required=True),
    "name": fields.Str(required=False)
}, location="json")
def get_num_tokens(req: Request, message: dict, model_name: str):
    try:
        return {
            "code": 200,
            "num_tokens": num_tokens_from_messages([message], model=model_name)
        }
    except Exception as e:
        logger.error(traceback.format_exc())
        return {
            "code": 500,
            "msg": "{}".format(e)
        }


def _get_encoding(model: str):
    if model in encoding_cache:
        return encoding_cache[model]
    try:
        encoding = tiktoken.encoding_for_model(model)
    except KeyError:
        encoding = tiktoken.get_encoding("cl100k_base")
    encoding_cache[model] = encoding
    return encoding


def num_tokens_from_messages(messages, model="deepseek-v4-flash"):
    """Returns the number of tokens used by a list of messages."""
    encoding = _get_encoding(model)
    num_tokens = 0
    for message in messages:
        num_tokens += 4
        for key, value in message.items():
            num_tokens += len(encoding.encode(str(value)))
            if key == "name":
                num_tokens -= 1
    num_tokens += 3
    return num_tokens
