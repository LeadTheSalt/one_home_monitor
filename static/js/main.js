// Data treatmen functions
function scale_this_date(date,scall_dist){
    h = new Date(date*1000);
    if (scall_dist > 60) {h.setSeconds(0);}
    if (scall_dist > 60*60) {h.setMinutes(0);}
    if (scall_dist > 60*60*2) {h.setHours(parseInt((h.getHours)/2) * 2);}
    if (scall_dist > 60*60*3) {h.setHours(parseInt((h.getHours)/3) * 3);}
    if (scall_dist > 60*60*6) {h.setHours(parseInt((h.getHours)/6) * 6);}
    if (scall_dist > 60*60*12) {h.setHours(parseInt((h.getHours)/12) * 12);}
    if (scall_dist > 60*60*24) {h.setHours(0);}
    if (scall_dist > 60*60*24*3) {h.setDate(parseInt((h.getDate)/3) * 3);}
    // no point going futher
    return h/1000
}
function mean(tab){
    c = 0; s = 0;
    for (i in tab){s += parseFloat(tab[i]);c++}
    return Math.round((s/c)* 10) / 10
}
function treat_data(readings, from, to,  nb_points){
    // Find last temp/hu/pre and rediuse to nb_points
    last = 0
    scall_dist = (to - from) / nb_points 
    // depending on the scale_distance find the best agg 
    agg_te = {}; agg_pr = {}; agg_hu = {}; scalled_dates = [], label_dates = []
    for (var date in readings){
        if (last < date) { last = date} //find last date in data
        h = scale_this_date(date,scall_dist)
        if ( scalled_dates.includes(h)) {
            agg_te[h].push(readings[date]["Te"])
            agg_pr[h].push(readings[date]["Pr"])
            agg_hu[h].push(readings[date]["Hu"])
        } else {
            agg_te[h] = [readings[date]["Te"]]
            agg_pr[h] = [readings[date]["Pr"]]
            agg_hu[h] = [readings[date]["Hu"]]
            scalled_dates.push(h)
        }
    }
    agg_te_out = [], agg_pr_out =[], agg_hu_out =[]
    for (const date of scalled_dates){
        agg_te_out.push(mean(agg_te[date]))
        agg_pr_out.push(mean(agg_pr[date]))
        agg_hu_out.push(mean(agg_hu[date]))
        l = new Date(date*1000)
        l = l.toDateString() + " " + l.toLocaleTimeString('fr-FR').split(':')[0]+":"+l.toLocaleTimeString('fr-FR').split(':')[1]
        label_dates.push(l)
    }
    out_res = {
        'last_te': readings[last]["Te"],
        'last_pr': readings[last]["Pr"],
        'last_hu': readings[last]["Hu"],
        'points_te': agg_te_out,
        'points_pr': agg_pr_out,
        'points_hu':agg_hu_out,
        'label_dates':label_dates,
    }
    return out_res
}

// Layout functions 
function layout_add_card(grid_div, txt_value, txt_unit, txt_name){
    var new_card = document.createElement("div");
    new_card.classList.add("uk-card");
    new_card.classList.add("uk-card-default");
    new_card.classList.add("uk-card-body");
    grid_div.appendChild(new_card);
    var inside_card = document.createElement("div");
    var value = document.createElement("span");
    value.classList.add("uk-text-large");
    value.classList.add("uk-text-bolder");
    value.appendChild(document.createTextNode(txt_value))
    var unit = document.createElement("span");
    unit.classList.add("uk-text-top");
    unit.appendChild(document.createTextNode(txt_unit))
    var name = document.createElement("span");
    name.appendChild(document.createTextNode(txt_name))
    inside_card.appendChild(value)
    inside_card.appendChild(unit)
    inside_card.appendChild(document.createElement("br"))
    inside_card.appendChild(name)
    new_card.appendChild(inside_card)
}
function create_layout(sensor,data){
    chart_div = document.getElementById('data_place_holder')
    // Add title of sensor
    var title_div = document.createElement("div");
    title_div.classList.add("uk-text-left");
    title_div.classList.add("uk-margin-bottom");
    var title_header = document.createElement("h2")
    title_header.classList.add("uk-heading-divider");
    title_header.appendChild(document.createTextNode(sensor.split('_')[1]));
    title_div.appendChild(title_header)
    chart_div.appendChild(title_div)
    // last info cards 
    var cards_div = document.createElement("div");
    cards_div.classList.add("uk-child-width-1-3@s");
    cards_div.classList.add("uk-grid-match");
    cards_div.classList.add("uk-grid-small");
    cards_div.classList.add("uk-text-center");
    cards_div.classList.add("uk-margin-top");
    cards_div.classList.add("uk-margin-bottom");
    cards_div.classList.add("uk-margin-left");
    cards_div.classList.add("uk-margin-right");
    cards_div.setAttribute("uk-grid", "");
    cards_div.id = "grid_" + sensor;
    layout_add_card(cards_div,data['last_te'],"°C","Temparature");
    layout_add_card(cards_div,data['last_pr'],"hPa","Pressure");
    layout_add_card(cards_div,data['last_hu'],"%","Hymidity");
    chart_div.appendChild(cards_div)  
    //Add chart
    var chart = document.createElement("canvas");
    chart.id = sensor; chart_div.appendChild(chart);
    temp_dataset = {
        label: 'Temparatures (°C)',
        yAxisID: 'T',
        fill: false,
        backgroundColor: "rgb(25, 121, 169)",
        borderColor: "rgb(25, 121, 169)",
        data: data['points_te'],
    }
    pres_dataset = {
        label: 'Pressure (hPa)',
        yAxisID: 'P',
        fill: false,
        backgroundColor: "rgb(224, 123, 57)",
        borderColor: "rgb(224, 123, 57)",
        data: data['points_pr'],
    }
    var ctx = document.getElementById(sensor).getContext('2d');
    var chart = new Chart(ctx, {
        type: 'line',
        data: {
            labels: data['label_dates'],
            datasets: [temp_dataset, pres_dataset]
        },
        options: {
            legend: {
                display: true,
                position: "bottom"
            },
            scales: {
                yAxes: [{
                    id: 'T',
                    type: 'linear',
                    position: 'left',
                }, {
                    id: 'P',
                    type: 'linear',
                    position: 'right',
                }],
                xAxes: [{
                    ticks: {
                        autoSkip: true,
                        maxTicksLimit: 15
                    }
                }]
            }
        }
    });
}

// Apps functions
function load_data_to_page(from,to,nb_points){
    from = Math.floor(from/1000)
    to = Math.floor(to/1000)
    query_url = '/sensordata?f=' + from 
    $.getJSON(query_url, function(data) {
        for (var sensor in data) {
            data = treat_data(data[sensor], from, to, nb_points)
            create_layout(sensor,data)
        }
    })
}

// TODO: Creat presets
// TODO: Creat functions called on button


function main (){
    // Load data for the frist time 
    var to = new Date()
    var from = new Date();
    from.setDate(from.getDate() - 5);
    load_data_to_page(from,to,100)
    // Remove spinner
    document.getElementById("load_spinner").remove();
    document.getElementById("load_msg").remove();
}
// call main function when the page is loaded 
window.onload = function() {main()}