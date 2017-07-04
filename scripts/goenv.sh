gopkg='gitlab.codility.net/marcink/redis-proxy' # TODO: automatic detection
repo_root="$(git rev-parse --show-toplevel)"
export GOPATH="${repo_root}/.gopath"
export GOBIN="${GOPATH}/bin"

gopkg_path="${GOPATH}/src/${gopkg}"
if ! [ -L "${gopkg_path}" ] ; then
    mkdir -p "${gopkg_path%/*}"
    ln -sf "${repo_root}" "${gopkg_path}"
fi

subdir="$(pwd)"
subdir="${subdir#$repo_root}"
cd "${gopkg_path}/${subdir}"
