function addServer() {
	const sendname = document.getElementById("addserver-name").value
		  sendip = document.getElementById("addserver-ip").value
		  XHR = new XMLHttpRequest(),
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

function processForm() {
	
}
