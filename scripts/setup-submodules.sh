#!/usr/bin/env bash
set -euo pipefail

# Setup external repos with optional sparse checkout
# Reads configuration from submodules.yaml
#
# NOTE: These are NOT git submodules. They are regular clones into
# gitignored directories for investigation purposes.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
CONFIG="$ROOT_DIR/submodules.yaml"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

log_info() { echo -e "${GREEN}==>${NC} $1"; }
log_warn() { echo -e "${YELLOW}==>${NC} $1"; }
log_error() { echo -e "${RED}==>${NC} $1"; }

GITIGNORE="$ROOT_DIR/.gitignore"

# Ensure a directory is listed in .gitignore
ensure_gitignore() {
    local name="$1"
    local entry="/${name}/"

    # Create .gitignore if it doesn't exist
    [[ -f "$GITIGNORE" ]] || touch "$GITIGNORE"

    # Check if entry already exists (exact line match)
    if ! grep -qxF "$entry" "$GITIGNORE"; then
        # Append under a managed section header if not already present
        local header="# Cloned submodule repos (managed by setup-submodules.sh)"
        if ! grep -qxF "$header" "$GITIGNORE"; then
            printf '\n%s\n' "$header" >> "$GITIGNORE"
        fi
        echo "$entry" >> "$GITIGNORE"
        log_info "  Added $entry to .gitignore"
    fi
}

# Check dependencies
check_deps() {
    if ! command -v yq >/dev/null 2>&1; then
        log_error "yq is required but not installed."
        echo "  Install with: brew install yq"
        exit 1
    fi
    
    if ! command -v git >/dev/null 2>&1; then
        log_error "git is required but not installed."
        exit 1
    fi
}

# Setup a single repo
setup_repo() {
    local name="$1"
    
    log_info "Setting up: $name"
    
    # Ensure the clone target is gitignored
    ensure_gitignore "$name"
    
    local url branch shallow has_sparse mode
    url=$(yq ".submodules.\"$name\".url" "$CONFIG")
    branch=$(yq ".submodules.\"$name\".branch // \"main\"" "$CONFIG")
    shallow=$(yq ".submodules.\"$name\".shallow // false" "$CONFIG")
    has_sparse=$(yq ".submodules.\"$name\".sparse != null" "$CONFIG")
    
    local target_dir="$ROOT_DIR/$name"
    
    # Clone if not exists
    if [[ ! -d "$target_dir/.git" ]]; then
        log_info "  Cloning from $url..."
        
        local clone_args=(--branch "$branch" --single-branch)
        [[ "$shallow" == "true" ]] && clone_args+=(--depth 1)
        
        # If sparse checkout, do a no-checkout clone first
        if [[ "$has_sparse" == "true" ]]; then
            clone_args+=(--no-checkout --filter=blob:none)
        fi
        
        git clone "${clone_args[@]}" "$url" "$target_dir"
    else
        log_info "  Already cloned, fetching latest..."
        git -C "$target_dir" fetch origin "$branch" || log_warn "  Fetch failed, continuing..."
        git -C "$target_dir" checkout "$branch" 2>/dev/null || true
        git -C "$target_dir" pull --ff-only 2>/dev/null || log_warn "  Pull failed (may have local changes)"
    fi
    
    # Configure sparse checkout if specified
    if [[ "$has_sparse" == "true" ]]; then
        mode=$(yq ".submodules.\"$name\".sparse.mode // \"cone\"" "$CONFIG")
        
        log_info "  Configuring sparse checkout (mode: $mode)..."
        
        # Build list of patterns for sparse-checkout
        local patterns=()
        local has_files=false
        
        # Get count of paths
        local path_count
        path_count=$(yq ".submodules.\"$name\".sparse.paths | length" "$CONFIG" 2>/dev/null || echo "0")
        
        if [[ "$path_count" -gt 0 ]]; then
            for ((i=0; i<path_count; i++)); do
                # Check if this is a string or an object
                local is_string
                is_string=$(yq ".submodules.\"$name\".sparse.paths[$i] | type" "$CONFIG" 2>/dev/null || echo "!!null")
                
                if [[ "$is_string" == "!!str" ]]; then
                    # Simple string path - include entire directory
                    local path
                    path=$(yq ".submodules.\"$name\".sparse.paths[$i]" "$CONFIG")
                    patterns+=("dir:$path")
                else
                    # Object with path and files
                    local base_path
                    base_path=$(yq ".submodules.\"$name\".sparse.paths[$i].path" "$CONFIG" 2>/dev/null || echo "")
                    
                    if [[ -n "$base_path" && "$base_path" != "null" ]]; then
                        # Get files for this path
                        local file_count
                        file_count=$(yq ".submodules.\"$name\".sparse.paths[$i].files | length" "$CONFIG" 2>/dev/null || echo "0")
                        
                        if [[ "$file_count" -gt 0 ]]; then
                            has_files=true
                            for ((j=0; j<file_count; j++)); do
                                local file
                                file=$(yq ".submodules.\"$name\".sparse.paths[$i].files[$j]" "$CONFIG")
                                patterns+=("file:$base_path/$file")
                            done
                        fi
                    fi
                fi
            done
        fi
        
        # Warn if files specified with cone mode
        if [[ "$has_files" == "true" && "$mode" != "no-cone" ]]; then
            log_warn "  Individual files only work with mode: no-cone. Files will be ignored."
            # Filter out file patterns
            local filtered_patterns=()
            for pattern in "${patterns[@]}"; do
                if [[ "$pattern" == dir:* ]]; then
                    filtered_patterns+=("$pattern")
                fi
            done
            patterns=("${filtered_patterns[@]}")
        fi
        
        if [[ "$mode" == "no-cone" ]]; then
            # Non-cone mode: only include exactly what's specified (no root files)
            git -C "$target_dir" config core.sparseCheckout true
            # Overwrite sparse-checkout file with only our patterns (leading slash = root only)
            : > "$target_dir/.git/info/sparse-checkout"  # truncate
            
            if [[ "${#patterns[@]}" -gt 0 ]]; then
                for pattern in "${patterns[@]}"; do
                    if [[ "$pattern" == dir:* ]]; then
                        echo "/${pattern#dir:}/" >> "$target_dir/.git/info/sparse-checkout"
                    elif [[ "$pattern" == file:* ]]; then
                        echo "/${pattern#file:}" >> "$target_dir/.git/info/sparse-checkout"
                    fi
                done
            fi
        else
            # Cone mode: includes root files + specified directories
            git -C "$target_dir" sparse-checkout init --cone
            
            # Extract just directory paths for cone mode
            local dir_paths=()
            for pattern in "${patterns[@]}"; do
                if [[ "$pattern" == dir:* ]]; then
                    dir_paths+=("${pattern#dir:}")
                fi
            done
            
            if [[ "${#dir_paths[@]}" -gt 0 ]]; then
                git -C "$target_dir" sparse-checkout set "${dir_paths[@]}"
            fi
        fi
        
        # Checkout after setting sparse paths
        git -C "$target_dir" checkout "$branch" 2>/dev/null || true
        
        log_info "  Sparse checkout:"
        if [[ "${#patterns[@]}" -gt 0 ]]; then
            for pattern in "${patterns[@]}"; do
                if [[ "$pattern" == dir:* ]]; then
                    echo "    [dir]  ${pattern#dir:}"
                elif [[ "$pattern" == file:* ]]; then
                    echo "    [file] ${pattern#file:}"
                fi
            done
        fi
    fi
    
    echo ""
}

# Update an existing repo
update_repo() {
    local name="$1"
    local target_dir="$ROOT_DIR/$name"
    
    if [[ ! -d "$target_dir/.git" ]]; then
        log_warn "$name not cloned yet. Run without --update first."
        return
    fi
    
    log_info "Updating: $name"
    
    local branch
    branch=$(yq ".submodules.\"$name\".branch // \"main\"" "$CONFIG")
    
    git -C "$target_dir" fetch origin "$branch"
    git -C "$target_dir" checkout "$branch" 2>/dev/null || true
    git -C "$target_dir" pull --ff-only || log_warn "  Pull failed (may have local changes)"
    
    echo ""
}

# Clean a repo (remove it entirely)
clean_repo() {
    local name="$1"
    local target_dir="$ROOT_DIR/$name"
    
    if [[ -d "$target_dir" ]]; then
        log_info "Removing: $name"
        rm -rf "$target_dir"
    else
        log_warn "$name not found, skipping"
    fi
}

usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --update    Update existing repos instead of full setup"
    echo "  --clean     Remove all cloned repos"
    echo "  --help      Show this help"
    echo ""
    echo "Without options, clones repos that don't exist and updates those that do."
}

main() {
    local mode="setup"
    
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --update) mode="update"; shift ;;
            --clean) mode="clean"; shift ;;
            --help) usage; exit 0 ;;
            *) log_error "Unknown option: $1"; usage; exit 1 ;;
        esac
    done
    
    check_deps
    
    if [[ ! -f "$CONFIG" ]]; then
        log_error "Config file not found: $CONFIG"
        exit 1
    fi
    
    log_info "Reading config from: $CONFIG"
    echo ""
    
    # Get all repo names
    local repos=()
    while IFS= read -r name; do
        repos+=("$name")
    done < <(yq '.submodules | keys | .[]' "$CONFIG")
    
    case "$mode" in
        setup)
            for name in "${repos[@]}"; do
                setup_repo "$name"
            done
            log_info "Done! Repos are cloned into gitignored directories."
            ;;
        update)
            for name in "${repos[@]}"; do
                update_repo "$name"
            done
            log_info "Done updating."
            ;;
        clean)
            for name in "${repos[@]}"; do
                clean_repo "$name"
            done
            log_info "Done cleaning."
            ;;
    esac
}

main "$@"
