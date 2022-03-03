function addServer(sendname, sendip) {
	const XHR = new XMLHttpRequest()
		  FD = new FormData();
	FD.append("name", sendname)
	FD.append("ip", sendip)
	XHR.addEventListener('load', function(event) {
		alert(event);
		console.log(event);
	});
	XHR.addEventListener('error', function(event) {
		alert(event);
		console.log(event);
	});
	XHR.open('POST', 'http://'+window.location.host+'/api/servers');
	XHR.send(FD);
}

function addDimension(sendname, sendalias, sendserver) {
	const XHR = new XMLHttpRequest()
		  FD = new FormData();
	FD.append("name", sendname)
	FD.append("alias", sendalias)
	FD.append("server", sendserver)
	XHR.addEventListener('load', function(event) {
		alert(event);
		console.log(event);
	});
	XHR.addEventListener('error', function(event) {
		alert(event);
		console.log(event);
	});
	XHR.open('POST', 'http://'+window.location.host+'/api/dims');
	XHR.send(FD);
}
