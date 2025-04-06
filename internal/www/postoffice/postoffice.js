async function processResult(res) {
  const mimeType = res.headers.get('Content-Type').split(';')[0]
  if (mimeType == 'multipart/form-data') {
    const fd = await res.formData()
    displayPostcard(fd.values())
  } else {
    const blob = await res.blob()
    const file = new File([blob], extractFilename(res), { type: blob.type })
    downloadFile(file)
    document.querySelector('#output .code').classList.toggle('irrelevant', true)
    document.querySelector('#output .code-explain').classList.toggle('irrelevant', true)
    document.querySelector('#output').classList.toggle('loading', false)
  }
}

function extractFilename(res) {
  const cdh = res.headers.get('Content-Disposition')
  const filenameRegex = /filename=(?:(["'])([^;\n]+?)\1)/i;
  const matches = filenameRegex.exec(cdh);
  if (!matches) {
    return
  }

  return matches[2]
}

async function onlyOk(res) {
  if (!res.ok) {
    const body = await res.text().catch()
    if (res.status == 400) {
      throw new Error(body)
    } else {
      throw new Error(`Unusable HTTP response (${res.status}): ${body}`)
    }
    
  }
  return res
}

async function displayPostcard(files) {
  let image, html

  for (file of files) {
    switch(file.type) {
      case "text/css":
        await file.text().then(insertPostcardCSS)
        break;
      case "text/html":
        html = await file.text()
        break;
      case "image/jpeg":
      case "image/png":
        image =  {
          filename: file.name,
          blobURL: URL.createObjectURL(file),
        }
        break;
    }
  }

  insertPostcardHTML(html, image)
  const outClass = document.querySelector('#output').classList
  outClass.toggle('loading', false)
}


function insertPostcardCSS(css) {
  const outCSS = document.querySelector('#output style')
  outCSS.textContent = css;
}

function insertPostcardHTML(html, image) {
  const showHTML = document.querySelector('#output code')
  const outHTML = document.querySelector('#output div.postcard-html')
  const outDownload = document.querySelector('#output a.postcard-download')

  const [_, uniqueHTML] = html.split("\n\n")
  if (!uniqueHTML) {
    throw new Error("Unexpected HTML returned from postcard generator")
  }
  showHTML.textContent = uniqueHTML
  document.querySelector('#output .code').classList.toggle('irrelevant', false)
  document.querySelector('#output .code-explain').classList.toggle('irrelevant', false)

  outHTML.innerHTML = uniqueHTML.replaceAll(image.filename, image.blobURL)
  outDownload.href = image.blobURL
  outDownload.download = image.filename
}


function downloadFile(file) {
  const url = URL.createObjectURL(file);

  const a = document.createElement('a');
  a.href = url;
  a.download = file.name;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

function sideImageChanged(e) {
  const input = e.target
  const file = input.files[0]
  const reader = new FileReader();

  reader.addEventListener('load', (file) => {
    const img = new Image();

    img.addEventListener('load', () => {
      const width = img.width;
      const height = img.height;

      if (Math.abs(width - height) < 10) {
        input.dataset.orientation = "square"
      } else if (width > height) {
        input.dataset.orientation = "landscape"
      } else {
        input.dataset.orientation = "portrait"
      }

      input.setCustomValidity("")
      showAppropriateFlips()
    })

    img.addEventListener('error', () => {
      input.setCustomValidity("Doesn't appear to be an image file")
    })

    // Trigger loading the image
    img.src = file.target.result

    const label = document.querySelector(`label[for="${input.id}"]`)
    label.innerHTML = ''
    label.appendChild(img)

    allowSecrets(img)
  })

  reader.addEventListener('error', () => {
    input.setCustomValidity("Couldn't read the chosen file")
  })
  
  reader.readAsDataURL(file)
}

function allowSecrets(img) {
  const label = img.parentElement
  img.draggable = false

  const stopFileChooser = (e) => { e.preventDefault() }

  let dragged, startPoint;
  img.addEventListener('mousedown', (e) => {
    label.addEventListener('click', stopFileChooser)

    dragged = document.createElement('div')
    dragged.classList.add('secret')

    const rect = label.getBoundingClientRect()
    startPoint = {
      x: e.clientX - rect.left,
      y: e.clientY - rect.top,
    }
    dragged.style.left = `${startPoint.x}px`
    dragged.style.top = `${startPoint.y}px`

    img.parentElement.appendChild(dragged)

    const endDrag = (e) => {
      if (!dragged) return;
      
      const rect = label.getBoundingClientRect()
      const imgRect = img.getBoundingClientRect()
      const b = makeSecretBox(startPoint, {
        x: e.clientX - rect.left,
        y: e.clientY - rect.top,
      })
      // Translate coords relative & scaled to image, as needed
      const imgBox = {
        type: 'box',
        left: (b.left - (imgRect.left - rect.left)) / imgRect.width,
        top: (b.top - (imgRect.top - rect.top)) / imgRect.height,
        width: b.width / imgRect.width,
        height: b.height / imgRect.height,
      }
      const sideName = label.htmlFor.split('-')[0]

      dragged.innerHTML = `<input type="hidden" name="${sideName}.secrets">`
      dragged.querySelector('input').value = JSON.stringify(imgBox)

      // Wait a moment, or the value isn't set properly (?!)
      setTimeout(() => { dragged = null }, 1)
      
      window.removeEventListener('mouseup', endDrag)
    }

    window.addEventListener('mouseup', endDrag)
  })

  img.addEventListener('mousemove', (e) => {
    if (!dragged) return;

    const rect = label.getBoundingClientRect()
    const b = makeSecretBox(startPoint, {
      x: e.clientX - rect.left,
      y: e.clientY - rect.top,
    })

    dragged.style.left = `${b.left}px`
    dragged.style.top = `${b.top}px`
    dragged.style.width = `${b.width}px`
    dragged.style.height = `${b.height}px`
  })
}

const makeSecretBox = (startPoint, thisPoint) => ({
  left: Math.min(startPoint.x, thisPoint.x),
  top: Math.min(startPoint.y, thisPoint.y),
  width: Math.abs(startPoint.x - thisPoint.x),
  height: Math.abs(startPoint.y - thisPoint.y),
})

function showAppropriateFlips() {
  const frontOrientation = document.getElementById('front-image')?.dataset?.orientation
  const backOrientation = document.getElementById('back-image')?.dataset?.orientation
  const flipChoices = document.querySelectorAll('input[name="flip"]')

  if (!frontOrientation) {
    flipChoices.forEach((flip) => flip.parentElement.classList.toggle('irrelevant', true))
  } else if (!backOrientation) {
    flipChoices.forEach((flip) => flip.parentElement.classList.toggle('irrelevant', flip.value != "none"))
  } else if (frontOrientation == "square") {
    flipChoices.forEach((flip) => flip.parentElement.classList.toggle('irrelevant', false))
  } else {
    const homoriented = frontOrientation == backOrientation
    flipChoices.forEach((flip) => {
      const isIrrelevant = flip.value == 'none' || flip.value.endsWith('-hand') == homoriented
      flip.parentElement.classList.toggle('irrelevant', isIrrelevant)
    })
  }

  const chosen = document.querySelector('input[name="flip"]:checked')
  if (!chosen || chosen.parentElement.classList.contains('irrelevant')) {
    const first = document.querySelector('label:has(input[name="flip"]):not(.irrelevant) input')
    if (first) {
      first.checked = true
    }
  }
}

function showError(err) {
  console.error("Unable to create postcard:", err)

  const error = document.getElementById('error')
  const output = document.getElementById('output')

  error.querySelector('p').textContent = err.message
  output.classList.remove('loading')
  output.classList.add('irrelevant')
  error.classList.remove('irrelevant')
}

function removeError() {
  const error = document.getElementById('error')
  error.classList.add('irrelevant')
}

document.addEventListener("DOMContentLoaded", () => {
  const beginBtn = document.getElementById('begin')
  const form = document.getElementById('postcard-input')
  const output = document.getElementById('output')
  
  form.addEventListener('submit', async (event) => {
    event.preventDefault()

    form.classList.add('irrelevant')
    output.classList.remove('irrelevant')
    output.classList.add('loading')
    removeError()
    beginBtn.innerHTML = beginBtn.dataset.afterClick

    const {action, method} = event.target
    const body = new FormData(event.target)

    console.log("Requesting Postcard…")
    const res = await fetch(action, { method, body,})
      .then(onlyOk)
      .then(processResult)
      .then(() => console.log("Postcard retrieved & processed"))
      .catch(showError)
  })

  beginBtn.addEventListener('click', (e) => {
    form.classList.toggle('irrelevant', false)
    output.classList.toggle('irrelevant', true)
    removeError()
  })

  document.querySelectorAll('#front-image,#back-image')
    .forEach((input) => {
      input.addEventListener('change', sideImageChanged)
    })
  showAppropriateFlips()
})

if (navigator.serviceWorker) {
  navigator.serviceWorker.register('postoffice-serviceworker.js')
    .then(() => console.log("Service worker loaded: Postcards will be generated locally"))
    .catch(console.error)
} else {
  console.warn("Service Workers are not available. You must access this page over HTTPS, or on localhost.")
}

