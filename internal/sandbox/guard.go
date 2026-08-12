package sandbox

// guardSource Python 安全壳：在用户代码之前于子进程内执行。
// 防护设计（关键约束：importlib 内部用 builtins.exec 执行模块代码，exec 不能全局替换）：
//  1. 受限 open：仅允许沙箱工作目录内文件（pandas.read_csv 等库内部依赖 open）
//  2. os 危险函数清理（system/popen/spawn/exec 等，模块对象全局共享）
//  3. 模块导入黑名单（网络/进程/二进制操作类）
//  4. 用户代码命名空间注入受限 builtins 白名单：exec/eval/compile/input 等根本不可见
//
// 用户代码经 stdin 传入，异常捕获后转为 error 事件，结果协议行总是输出。
const guardSource = `# DocMind Python 沙箱安全壳（guard）
# 边界声明：薄壳防"失控/误用"，不防有意的对抗代码；兜底依赖：
# 单租户 + 脚本输入服务端可控 + 环境无敏感信息 + 网络/子进程被禁。
import base64
import builtins
import email.message
import importlib
import io
import json
import os
import pathlib
import sys
import traceback
import urllib.error
import urllib.parse

# 预导入第三方数据分析库（裁剪前完成）：库的完整依赖链（含 socket/subprocess 等
# 危险模块）在导入黑名单生效前全部加载进 sys.modules，避免 importlib 加载库时被
# 黑名单误伤；裁剪后用户再 import 危险模块会重新加载并触发黑名单拦截，
# 已加载库的内部引用不受影响。代价：每次沙箱执行增加约 1-2s 启动开销。
for _lib in ("numpy", "pandas", "duckdb", "matplotlib"):
    try:
        importlib.import_module(_lib)
    except Exception as _e:
        # 未安装的库允许缺失（用户代码 import 时获得原始错误），但失败原因输出到 stderr 便于排查
        print(f"[guard] 预导入 {_lib} 失败: {_e}", file=sys.stderr)

try:
    # pyplot 及其依赖链（backend_bases→socket、font_manager→subprocess）预加载，
    # 否则用户 import matplotlib.pyplot 时重新加载会触发黑名单拦截
    importlib.import_module("matplotlib.pyplot")
except Exception as _e:
    print(f"[guard] 预导入 matplotlib.pyplot 失败: {_e}", file=sys.stderr)

sys.stdin.reconfigure(encoding="utf-8")
sys.stdout.reconfigure(encoding="utf-8")
sys.stderr.reconfigure(encoding="utf-8")

_WORKDIR = os.getcwd()


def _deny(name):
    def _f(*args, **kwargs):
        raise PermissionError(f"沙箱禁止调用 {name}()")
    return _f


# 1. 受限 open：仅允许沙箱工作目录内的文件（pandas.read_csv 内部依赖 open，不能整禁）
#    额外放行系统字体目录（matplotlib 需扫描系统字体渲染中文标签；字体文件非敏感数据）
_orig_open = builtins.open

if os.name == "nt":
    _FONT_DIRS = (os.environ.get("WINDIR", "C:\\Windows") + "\\Fonts",)
else:
    _FONT_DIRS = ("/usr/share/fonts", "/usr/local/share/fonts", "/etc/fonts")


def _sandbox_open(file, mode="r", *args, **kwargs):
    if isinstance(file, (str, bytes, os.PathLike)):
        target = os.path.abspath(os.fspath(file))
        if target == _WORKDIR or target.startswith(_WORKDIR + os.sep):
            return _orig_open(file, mode, *args, **kwargs)
        for _d in _FONT_DIRS:
            if target.startswith(_d + os.sep):
                return _orig_open(file, mode, *args, **kwargs)
        raise PermissionError(f"沙箱禁止访问工作目录外的文件: {file!r}")
    return _orig_open(file, mode, *args, **kwargs)


# 2. 清理 os 模块危险函数（模块对象全局共享，所有库同时生效）
for _fn in ("system", "popen", "spawnl", "spawnle", "spawnlp", "spawnlpe",
            "spawnv", "spawnve", "spawnvp", "spawnvpe", "fork", "forkpty",
            "execv", "execve", "execvp", "execvpe", "execl", "execle",
            "execlp", "execlpe", "posix_spawn", "posix_spawnp",
            "startfile", "kill", "killpg", "setuid", "setgid", "chroot", "open"):
    if hasattr(os, _fn):
        setattr(os, _fn, _deny(f"os.{_fn}"))

# 3. 模块导入黑名单（网络/进程/二进制操作；os/sys/io/importlib 等库依赖模块不禁）
#    注意子模块粒度：urllib.parse 是 pathlib 的依赖（duckdb→importlib.metadata→pathlib），
#    只能禁带网络能力的 urllib.request，不能整包禁 urllib。
_DENIED_MODULES = frozenset({
    "socket", "subprocess", "ctypes", "multiprocessing",
    "requests", "http", "httpx", "aiohttp", "urllib3",
    "ftplib", "smtplib", "poplib", "imaplib", "telnetlib", "nntplib",
    "xmlrpc", "webbrowser", "pty", "pexpect", "paramiko",
    "select", "selectors", "asyncio",
})
# 子模块精确黑名单（顶层包可导入，但网络能力子模块禁止）
_DENIED_SUBMODULES = frozenset({
    "urllib.request", "urllib.response",
})


def _guard_import(name, globals=None, locals=None, fromlist=(), level=0):
    if name.split(".")[0] in _DENIED_MODULES:
        raise ImportError(f"沙箱禁止导入模块 {name!r}")
    if name in _DENIED_SUBMODULES:
        raise ImportError(f"沙箱禁止导入模块 {name!r}")
    return _orig_import(name, globals, locals, fromlist, level)


_orig_import = builtins.__import__
builtins.__import__ = _guard_import

# 移除已预导入网络模块的 sys.modules 条目：库的模块对象引用不受影响，
# 用户再次 import 时重新加载并触发黑名单拦截
for _m in tuple(_DENIED_MODULES) + tuple(_DENIED_SUBMODULES):
    sys.modules.pop(_m, None)

# 4. 用户命名空间：受限 builtins 白名单（不含 exec/eval/compile/input/globals 等）
#    importlib 内部 exec 模块代码依赖全局 builtins.exec，不能全局替换；
#    改为在用户命名空间白名单中排除，用户代码查找不到即可。
_SAFE_BUILTINS = {}
for _name in ("abs", "all", "any", "ascii", "bool", "bytearray", "bytes", "callable",
              "chr", "classmethod", "complex", "delattr", "dict", "dir", "divmod",
              "enumerate", "filter", "float", "format", "frozenset", "getattr",
              "hasattr", "hash", "id", "int", "isinstance", "issubclass", "iter",
              "len", "list", "map", "max", "min", "next", "object",
              "oct", "ord", "pow", "print", "property", "range", "repr", "reversed",
              "round", "set", "setattr", "slice", "sorted", "staticmethod", "str",
              "sum", "super", "tuple", "type", "zip",
              "ArithmeticError", "AssertionError", "AttributeError", "BaseException",
              "Exception", "IndexError", "KeyError", "NameError", "NotImplemented",
              "NotImplementedError", "OverflowError", "RuntimeError", "StopIteration",
              "SyntaxError", "TypeError", "ValueError", "ZeroDivisionError"):
    _SAFE_BUILTINS[_name] = getattr(builtins, _name)
_SAFE_BUILTINS["open"] = _sandbox_open
_SAFE_BUILTINS["__import__"] = _guard_import

# 5. 结果输出 API（emit_*，供用户代码调用，统一序列化为事件）
_OUTPUTS = []

# matplotlib 中文字体：优先常见中文字体，缺失时回退 DejaVu Sans（中文将显示为方块）
try:
    import matplotlib as _mpl
    _mpl.rcParams["font.sans-serif"] = [
        "Microsoft YaHei", "SimHei", "Noto Sans CJK SC", "WenQuanYi Micro Hei", "DejaVu Sans",
    ]
    _mpl.rcParams["axes.unicode_minus"] = False
except Exception:
    pass


def emit_text(obj):
    _OUTPUTS.append({"type": "text", "content": str(obj)})


def emit_table(df, max_rows=50):
    columns = [str(c) for c in df.columns]
    rows = []
    for _, row in df.head(max_rows).iterrows():
        rows.append(["" if v is None else str(v) for v in row.tolist()])
    _OUTPUTS.append({"type": "table", "columns": columns, "rows": rows})


def emit_chart(fig):
    _buf = io.BytesIO()
    fig.savefig(_buf, format="png", bbox_inches="tight")
    _OUTPUTS.append({"type": "chart", "format": "png",
                     "data_base64": base64.b64encode(_buf.getvalue()).decode("utf-8")})


def emit_error(message):
    _OUTPUTS.append({"type": "error", "content": str(message)})


# 6. 执行用户代码：异常捕获后转为 error 事件，保证结果协议总是输出
def _main():
    code = sys.stdin.read()
    user_globals = {
        "__name__": "__main__",
        "__builtins__": _SAFE_BUILTINS,
        "emit_text": emit_text,
        "emit_table": emit_table,
        "emit_chart": emit_chart,
    }
    try:
        builtins.exec(compile(code, "<sandbox>", "exec"), user_globals)
    except SystemExit:
        pass
    except BaseException:
        emit_error(traceback.format_exc())
    finally:
        print("__SANDBOX_RESULT__" + json.dumps(_OUTPUTS, ensure_ascii=False))


_main()
`
