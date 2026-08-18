from setuptools import setup, Extension
from Cython.Build import cythonize
import os

extensions = [
    Extension(
        "mira_memory",
        sources=["mira_memory.pyx"],
        include_dirs=[os.path.abspath("../build/engine-memory-go")],
        library_dirs=[os.path.abspath("../build/engine-memory-go")],
        libraries=["mira_memory"],
        language="c++",
    )
]

setup(
    name="mira-memory",
    ext_modules=cythonize(extensions, language_level="3"),
)
