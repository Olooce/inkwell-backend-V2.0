package llm

/*
#cgo CFLAGS: -Iinternal/llm/deepfloyd_env/include/python3.10
#cgo LDFLAGS: -Linternal/llm/deepfloyd_env/lib -lpython3.10

#include <Python.h>
#include <stdlib.h>

static PyObject* pModule;
static PyObject* pGenerateFunc;

// Initialize Python and load DeepFloyd
int InitDeepFloyd() {
    Py_Initialize();
    if (!Py_IsInitialized()) {
        PyErr_Print();
        return -1;
    }

    PyObject *pName = PyUnicode_DecodeFSDefault("diffusers");
    if (!pName) {
        PyErr_Print();
        return -1;
    }

    pModule = PyImport_ImportModule("diffusers");
    Py_XDECREF(pName);

    if (!pModule) {
        PyErr_Print();
        return -1;
    }

    return 0;
}

// Generate image using DeepFloyd IF
char* GenerateImage(char *prompt) {
    PyRun_SimpleString("from diffusers import DiffusionPipeline\n"
                       "import torch\n"
                       "device = 'cuda' if torch.cuda.is_available() else 'cpu'\n"
                       "pipe = DiffusionPipeline.from_pretrained('DeepFloyd/IF-I-M-v1.0').to(device)\n"
                       "def generate_image(prompt):\n"
                       "    image = pipe(prompt).images[0]\n"
                       "    path = 'generated_image.png'\n"
                       "    image.save(path)\n"
                       "    return path\n");

    PyObject *pFunc = PyObject_GetAttrString(pModule, "generate_image");
    if (!pFunc || !PyCallable_Check(pFunc)) {
        PyErr_Print();
        return NULL;
    }

    PyObject *pArgs = PyTuple_Pack(1, PyUnicode_FromString(prompt));
    if (!pArgs) {
        PyErr_Print();
        return NULL;
    }

    PyObject *pValue = PyObject_CallObject(pFunc, pArgs);
    Py_XDECREF(pArgs);
    Py_XDECREF(pFunc);

    if (!pValue) {
        PyErr_Print();
        return NULL;
    }

    char *result = strdup(PyUnicode_AsUTF8(pValue));
    Py_XDECREF(pValue);

    return result;
}

// Cleanup
void CloseDeepFloyd() {
    Py_Finalize();
}
*/
import "C"
import (
	"fmt"
	"os/exec"
	"strings"
	"unsafe"
)

// DeepFloydWrapper handles DeepFloyd image generation
type DeepFloydWrapper struct{}

// Start initializes Python and loads DeepFloyd
func (d *DeepFloydWrapper) Start() error {
	if err := checkDependencies(); err != nil {
		return err
	}

	if C.InitDeepFloyd() != 0 {
		return fmt.Errorf("failed to initialize DeepFloyd")
	}
	return nil
}

// GenerateImage creates an image from a text prompt
func (d *DeepFloydWrapper) GenerateImage(prompt string) (string, error) {
	cPrompt := C.CString(prompt)
	defer C.free(unsafe.Pointer(cPrompt))

	cResult := C.GenerateImage(cPrompt)
	if cResult == nil {
		return "", fmt.Errorf("image generation failed")
	}

	result := C.GoString(cResult)
	C.free(unsafe.Pointer(cResult)) // Free allocated memory

	return result, nil
}

// Stop cleans up the Python environment
func (d *DeepFloydWrapper) Stop() {
	C.CloseDeepFloyd()
}

// checkDependencies verifies if Python headers and libraries are available
func checkDependencies() error {
	// Check if `pkg-config` is installed
	_, err := exec.LookPath("pkg-config")
	if err != nil {
		return fmt.Errorf("pkg-config not found. Install with:\n  sudo apt install pkg-config")
	}

	// Check available Python versions
	pythonVersionCmd := exec.Command("python3", "--version")
	pythonVersionOutput, err := pythonVersionCmd.Output()
	if err != nil {
		return fmt.Errorf("Python3 is not installed. Install with:\n  sudo apt install python3\nError: %s", err)
	}
	fmt.Println("Detected Python version:", strings.TrimSpace(string(pythonVersionOutput)))

	// Check if Python development headers exist
	_, err = exec.Command("pkg-config", "--exists", "python3").CombinedOutput()
	if err != nil {
		return fmt.Errorf("Python development headers are missing. Install with:\n  sudo apt install python3-dev")
	}

	return nil
}
