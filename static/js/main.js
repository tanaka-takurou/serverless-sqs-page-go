$(document).ready(function() {
  GetCount();
});

var GetCount = function() {
  var action = 'getcount';
  const data = {action};
  request(data, (res)=>{
    $("#number").text(res.message);
  }, (e)=>{
    console.log(e.responseJSON.message);
  });
};

var SubmitForm = function(action) {
  $(".submitbutton").addClass('disabled');
  var message = $('#message').val();
  if (action == "sendmessage" && !message) {
    $(".submitbutton").removeClass('disabled');
    $("#warning").text("Message is Empty").removeClass("hidden").addClass("visible");
    return false;
  }
  const data = {action, message};
  request(data, (res)=>{
    $("#result").text(res.message);
    $("#info").removeClass("hidden").addClass("visible");
    $(".submitbutton").removeClass('disabled');
  }, (e)=>{
    console.log(e.responseJSON.message);
    $("#warning").text(e.responseJSON.message).removeClass("hidden").addClass("visible");
    $(".submitbutton").removeClass('disabled');
  });
};

var request = function(data, callback, onerror) {
  $.ajax({
    type:          'POST',
    dataType:      'json',
    contentType:   'application/json',
    scriptCharset: 'utf-8',
    data:          JSON.stringify(data),
    url:           {{ .Api }}
  })
  .done(function(res) {
    callback(res);
  })
  .fail(function(e) {
    onerror(e);
  });
};
