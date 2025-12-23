package api

import (
	"net/http"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	gin "github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

func addCSS() gomponents.Node {
	return html.StyleEl(gomponents.Raw(`
					.sidebar-dropdown .sidebar-link:before {
    					content: unset;
					}
					.listunstyle { list-style-type: none; }
					.alert-column { flex-direction: column; }
					.config-section { 
						background: linear-gradient(135deg, #ffffff 0%, #f8f9fa 100%);
						border-radius: 12px;
						padding: 2rem;
						box-shadow: 0 4px 16px rgba(0,0,0,0.08);
						margin-bottom: 2.5rem;
						border: 1px solid #e9ecef;
					}
					.config-section h3 {
						color: #495057;
						font-weight: 700;
						margin-bottom: 1.5rem;
						font-size: 1.5rem;
					}
					.array-item, .array-item-enhanced { 
						background: linear-gradient(135deg, #ffffff 0%, #f8f9fa 100%);
						border: 2px solid #e3f2fd !important;
						border-radius: 12px;
						margin-bottom: 1.5rem;
						box-shadow: 0 4px 12px rgba(0,0,0,0.08);
						transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
						overflow: hidden;
					}
					.array-item:hover, .array-item-enhanced:hover {
						transform: translateY(-3px);
						box-shadow: 0 8px 25px rgba(13, 110, 253, 0.15);
						border-color: #0d6efd !important;
					}
					.array-item-enhanced .card-header {
						background: linear-gradient(135deg, #0d6efd 0%, #0056b3 100%) !important;
						color: white !important;
						font-weight: 600;
						padding: 1rem 1.5rem;
						border: none !important;
						transition: all 0.3s ease;
					}
					.array-item-enhanced .card-header:hover {
						background: linear-gradient(135deg, #0056b3 0%, #004085 100%) !important;
					}
					.array-item-enhanced .card-body {
						background: linear-gradient(135deg, #ffffff 0%, #f8f9fa 100%) !important;
						padding: 1.5rem;
						border: none !important;
					}
					.collapse {
						transition: height 0.35s cubic-bezier(0.4, 0, 0.2, 1) !important;
					}
					.collapsing {
						transition: height 0.35s cubic-bezier(0.4, 0, 0.2, 1) !important;
					}
					
					/* Enhanced Button Styling */
					.btn {
						font-weight: 600;
						border-radius: 8px;
						position: relative;
						overflow: hidden;
					}
					.btn::before {
						content: '';
						position: absolute;
						top: 0;
						left: -100%;
						width: 100%;
						height: 100%;
						background: linear-gradient(90deg, transparent, rgba(255,255,255,0.3), transparent);
						transition: left 0.5s;
					}
					.btn:hover::before {
						left: 100%;
					}
					
					/* Card hover animations */
					.card {
						transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
					}
					.card:hover {
						transform: translateY(-2px);
					}
					
					/* Accordion-style collapsing with smooth icon rotation */
					.card-header[data-bs-toggle="collapse"] {
						position: relative;
					}
					.card-header[data-bs-toggle="collapse"]:hover {
						background: linear-gradient(135deg, #0056b3 0%, #004085 100%) !important;
					}
					
					/* Loading states for HTMX requests */
					.htmx-request {
						opacity: 0.7;
						transition: opacity 0.3s ease;
					}
					.htmx-request::after {
						content: '';
						position: absolute;
						top: 50%;
						left: 50%;
						width: 20px;
						height: 20px;
						margin: -10px 0 0 -10px;
						border: 2px solid #0d6efd;
						border-radius: 50%;
						border-top-color: transparent;
						animation: spin 1s linear infinite;
					}
					
					@keyframes spin {
						to { transform: rotate(360deg); }
					}
					
					/* Enhanced form field focus animations */
					.form-control:focus, .form-select:focus {
						animation: focusPulse 0.3s ease-out;
					}
					
					@keyframes focusPulse {
						0% { transform: scale(1); }
						50% { transform: scale(1.02); }
						100% { transform: scale(1.02); }
					}
					
					/* Enhanced Sidebar Styling */
					.sidebar {
						background: linear-gradient(180deg, #1e3a5f 0%, #2c5282 50%, #1e3a5f 100%);
						box-shadow: 4px 0 20px rgba(0,0,0,0.15);
						border-right: 2px solid rgba(255,255,255,0.1);
					}
					
					.sidebar-brand {
						background: linear-gradient(135deg, #0d6efd 0%, #0056b3 100%);
						border-bottom: 2px solid rgba(255,255,255,0.2);
						padding: 1.5rem 1rem;
						transition: all 0.3s ease;
					}
					
					.sidebar-brand:hover {
						background: linear-gradient(135deg, #0056b3 0%, #004085 100%);
						transform: scale(1.02);
					}
					
					.sidebar-brand-text {
						color: white !important;
						font-weight: 700;
						font-size: 1.1rem;
						text-shadow: 0 2px 4px rgba(0,0,0,0.3);
					}
					
					.sidebar-header {
						color: rgba(255,255,255,0.7) !important;
						font-weight: 600;
						font-size: 0.75rem;
						text-transform: uppercase;
						letter-spacing: 1px;
						padding: 1rem 1.5rem 0.5rem;
						margin-top: 1rem;
						position: relative;
					}
					
					.sidebar-header::after {
						content: '';
						position: absolute;
						bottom: 0;
						left: 1.5rem;
						right: 1.5rem;
						height: 1px;
						background: linear-gradient(90deg, transparent, rgba(255,255,255,0.3), transparent);
					}
					
					.sidebar-item {
						margin: 0.25rem 0.75rem;
						border-radius: 8px;
						transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
					}
					
					.sidebar-item:hover {
						background: rgba(255,255,255,0.1);
						transform: translateX(4px);
					}
					
					.sidebar-item.active {
						background: linear-gradient(135deg, rgba(13, 110, 253, 0.8) 0%, rgba(0, 86, 179, 0.8) 100%);
						box-shadow: 0 4px 12px rgba(13, 110, 253, 0.3);
					}
					
					.sidebar-link {
						color: rgba(255,255,255,0.9) !important;
						padding: 0.75rem 1rem;
						display: flex;
						align-items: center;
						text-decoration: none !important;
						transition: all 0.3s ease;
						border-radius: 6px;
						position: relative;
						overflow: hidden;
					}
					
					.sidebar-link::before {
						content: '';
						position: absolute;
						top: 0;
						left: -100%;
						width: 100%;
						height: 100%;
						background: linear-gradient(90deg, transparent, rgba(255,255,255,0.1), transparent);
						transition: left 0.5s;
					}
					
					.sidebar-link:hover::before {
						left: 100%;
					}
					
					.sidebar-link:hover {
						color: white !important;
						background: rgba(255,255,255,0.15);
						padding-left: 1.25rem;
					}
					
					.sidebar-link i {
						font-size: 1.1rem;
						margin-right: 0.75rem;
						min-width: 20px;
						text-align: center;
						transition: all 0.3s ease;
					}
					
					.sidebar-link:hover i {
						transform: scale(1.1);
						color: #ffd700;
					}
					
					.sidebar-link span {
						font-weight: 500;
						font-size: 0.9rem;
					}
					
					/* Collapsible sections */
					.sidebar-dropdown {
						background: rgba(0,0,0,0.2);
						border-radius: 6px;
						margin: 0.25rem 0;
						padding: 0.25rem 0;
					}
					
					.sidebar-dropdown .sidebar-item {
						margin: 0.125rem 0.5rem;
					}
					
					.sidebar-dropdown .sidebar-link {
						padding: 0.5rem 1rem;
						font-size: 0.85rem;
						color: rgba(255,255,255,0.8) !important;
					}
					
					.sidebar-dropdown .sidebar-link i {
						font-size: 0.9rem;
						margin-right: 0.5rem;
					}
					
					/* Override any external CSS that adds arrows to sidebar links */
					.sidebar-link::after {
						content: '' !important;
						display: none !important;
					}
					
					/* Preserve the hover animation effect */
					.sidebar-link::before {
						content: '';
						position: absolute;
						top: 0;
						left: -100%;
						width: 100%;
						height: 100%;
						background: linear-gradient(90deg, transparent, rgba(255,255,255,0.1), transparent);
						transition: left 0.5s;
					}
					
					/* Sidebar badges */
					.sidebar-link .badge {
						font-size: 0.6rem !important;
						padding: 0.25rem 0.5rem;
						border-radius: 12px;
						font-weight: 600;
						animation: pulse 2s infinite;
					}
					
					@keyframes pulse {
						0% { opacity: 0.8; }
						50% { opacity: 1; }
						100% { opacity: 0.8; }
					}
					
					/* Enhanced sidebar scrollbar */
					.sidebar-content::-webkit-scrollbar {
						width: 6px;
					}
					
					.sidebar-content::-webkit-scrollbar-track {
						background: rgba(255,255,255,0.1);
						border-radius: 3px;
					}
					
					.sidebar-content::-webkit-scrollbar-thumb {
						background: linear-gradient(180deg, #0d6efd, #0056b3);
						border-radius: 3px;
					}
					
					.sidebar-content::-webkit-scrollbar-thumb:hover {
						background: linear-gradient(180deg, #0056b3, #004085);
					}
					
					/* Sidebar section separators */
					.sidebar-item.separator {
						border-top: 1px solid rgba(255,255,255,0.1);
						margin-top: 1rem;
						padding-top: 1rem;
					}
					
					/* Active page indicator */
					.sidebar-item.current {
						background: linear-gradient(135deg, rgba(255, 215, 0, 0.2) 0%, rgba(255, 193, 7, 0.2) 100%);
						border-left: 4px solid #ffd700;
					}
					
					.sidebar-item.current .sidebar-link {
						color: #ffd700 !important;
						font-weight: 600;
					}
					
					/* Responsive sidebar */
					@media (max-width: 768px) {
						.sidebar {
							transform: translateX(-100%);
							transition: transform 0.3s ease;
						}
						
						.sidebar.show {
							transform: translateX(0);
						}
					}
					
					/* Footer positioned only under main content, not sidebar */
					.content-footer {
						position: fixed;
						bottom: 0;
						left: 250px; /* Sidebar width */
						right: 0;
						z-index: 1000;
						transition: left 0.3s ease;
					}
					
					/* When sidebar is collapsed */
					.sidebar.collapsed ~ .main .content-footer {
						left: 0;
					}
					
					/* Responsive footer positioning */
					@media (max-width: 768px) {
						.content-footer {
							left: 0; /* Full width on mobile */
						}
					}
					
					/* Add bottom padding to main content to prevent footer overlap */
					.main {
						padding-bottom: 80px;
					}
					.array-item-header {
						display: flex;
						justify-content: between;
						align-items: center;
						margin-bottom: 1rem;
					}
					.btn-sm { font-size: 0.875rem; }
					.nested-array {
						border-left: 3px solid #0d6efd;
						padding-left: 1rem;
						margin-left: 1rem;
					}
					
					/* Enhanced Form Styling */
					.form-group-enhanced {
						position: relative;
					}
					.form-field-card:hover:not(.choices-open) {
						box-shadow: 0 2px 8px rgba(0,0,0,0.08) !important;
						border-color: #0d6efd !important;
					}
					.form-field-card:has(.choices.is-open) {
						box-shadow: 0 2px 8px rgba(0,0,0,0.08) !important;
						border-color: #0d6efd !important;
					}
					.form-check-wrapper:hover {
						transform: translateY(-1px);
						box-shadow: 0 2px 8px rgba(0,0,0,0.08);
						border-color: #0d6efd !important;
					}
					.form-control:focus, .form-select:focus {
						border-color: #0d6efd;
						box-shadow: 0 0 0 0.2rem rgba(13, 110, 253, 0.25);
						transform: scale(1.02);
						transition: all 0.2s ease;
					}
					.form-label {
						font-weight: 600;
						color: #495057;
						font-size: 0.95rem;
					}
					.form-text {
						font-size: 0.85rem;
						margin-top: 0.5rem;
					}
					.help-text-content {
						background: linear-gradient(135deg, #e3f2fd 0%, #f1f8e9 100%);
						border-left: 4px solid #2196f3 !important;
						border-radius: 6px !important;
						font-size: 0.875rem;
						line-height: 1.5;
					}
					.form-control, .form-select {
						border-radius: 6px;
						border: 1px solid #ced4da;
						transition: all 0.2s ease;
					}
					.form-control:hover, .form-select:hover {
						border-color: #6c757d;
					}
					.form-check-input:checked {
						background-color: #0d6efd;
						border-color: #0d6efd;
					}

					.form-check {
						padding: unset;
					}
						
					.form-switch {
						padding: unset;
					}
					
					/* Enhanced Database Interface Styling */
					.database-content-container {
						background: linear-gradient(135deg, #ffffff 0%, #f8f9fa 100%);
						border-radius: 16px;
						padding: 0;
						margin-bottom: 2rem;
						box-shadow: 0 8px 32px rgba(0,0,0,0.08);
						border: 1px solid rgba(255,255,255,0.2);
						overflow: hidden;
					}
					
					.database-header-card {
						background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
						padding: 2rem;
						color: white;
						position: relative;
						overflow: hidden;
					}
					
					.database-header-card::before {
						content: '';
						position: absolute;
						top: 0;
						left: 0;
						right: 0;
						bottom: 0;
						background: url('data:image/svg+xml,<svg width="60" height="60" viewBox="0 0 60 60" xmlns="http://www.w3.org/2000/svg"><g fill="none" fill-rule="evenodd"><g fill="%23ffffff" fill-opacity="0.03"><circle cx="30" cy="30" r="4"/></g></svg>');
						pointer-events: none;
					}
					
					.database-header-content {
						display: flex;
						justify-content: space-between;
						align-items: center;
						position: relative;
						z-index: 2;
					}
					
					.database-title-section {
						flex: 1;
					}
					
					.database-title {
						font-size: 2rem;
						font-weight: 700;
						margin: 0 0 0.5rem 0;
						text-shadow: 0 2px 4px rgba(0,0,0,0.1);
						display: flex;
						align-items: center;
					}
					
					.database-icon {
						background: rgba(255,255,255,0.2);
						padding: 0.5rem;
						border-radius: 12px;
						margin-right: 1rem;
						backdrop-filter: blur(10px);
					}
					
					.database-subtitle {
						margin: 0;
						opacity: 0.9;
						font-size: 1.1rem;
						font-weight: 400;
					}
					
					.database-actions {
						display: flex;
						gap: 1rem;
						align-items: center;
					}
					
					.btn-database-add, .btn-database-refresh {
						background: rgba(255,255,255,0.2);
						border: 1px solid rgba(255,255,255,0.3);
						color: white;
						padding: 0.75rem 1.5rem;
						border-radius: 12px;
						font-weight: 600;
						transition: all 0.3s ease;
						backdrop-filter: blur(10px);
						display: flex;
						align-items: center;
						gap: 0.5rem;
					}
					
					.btn-database-add:hover, .btn-database-refresh:hover {
						background: rgba(255,255,255,0.3);
						transform: translateY(-2px);
						box-shadow: 0 8px 25px rgba(0,0,0,0.15);
						color: white;
					}
					
					.btn-database-add i, .btn-database-refresh i {
						font-size: 0.9rem;
					}
					
					/* Enhanced Filters Card */
					.filters-card-enhanced {
						background: white;
						border-radius: 12px;
						margin: 1.5rem;
						box-shadow: 0 4px 16px rgba(0,0,0,0.08);
						border: 1px solid #e9ecef;
						overflow: hidden;
						transition: all 0.3s ease;
					}
					
					.filters-card-enhanced:hover {
						box-shadow: 0 8px 32px rgba(0,0,0,0.12);
					}
					
					.filters-header {
						background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%);
						padding: 1rem 1.5rem;
						border-bottom: 1px solid #dee2e6;
						display: flex;
						justify-content: space-between;
						align-items: center;
						cursor: pointer;
					}
					
					.filters-title-section {
						display: flex;
						align-items: center;
						gap: 0.75rem;
					}
					
					.filters-icon {
						background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
						color: white;
						padding: 0.5rem;
						border-radius: 8px;
						font-size: 0.9rem;
					}
					
					.filters-title {
						margin: 0;
						font-size: 1.1rem;
						font-weight: 600;
						color: #495057;
					}
					
					.filters-toggle-btn {
						background: none;
						border: none;
						color: #6c757d;
						font-size: 1.2rem;
						padding: 0.5rem;
						border-radius: 6px;
						transition: all 0.3s ease;
					}
					
					.filters-toggle-btn:hover {
						background: rgba(0,0,0,0.05);
						color: #495057;
					}
					
					.filters-body {
						padding: 1.5rem;
						transition: all 0.3s ease;
					}
					
					.filters-grid {
						display: grid;
						grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
						gap: 1rem;
						align-items: end;
					}
					
					.filters-body .form-label {
						font-weight: 600;
						color: #495057;
						margin-bottom: 0.5rem;
					}
					
					.filters-body .form-control {
						border-radius: 8px;
						border: 1px solid #ced4da;
						transition: all 0.3s ease;
					}
					
					.filters-body .form-control:focus {
						border-color: #667eea;
						box-shadow: 0 0 0 0.2rem rgba(102, 126, 234, 0.25);
					}
					
					/* Enhanced Table Container */
					.database-table-container {
						margin: 1.5rem;
						background: white;
						border-radius: 12px;
						overflow: hidden;
						box-shadow: 0 4px 16px rgba(0,0,0,0.08);
						border: 1px solid #e9ecef;
						padding: 1.5rem;
					}
					
					/* DataTables Controls Styling */
					.dataTables_wrapper {
						padding: 0;
					}
					
					.dataTables_wrapper .dataTables_length,
					.dataTables_wrapper .dataTables_filter {
						margin-bottom: 1.5rem;
						padding: 0;
					}
					
					.dataTables_wrapper .dataTables_length {
						float: left;
						margin-right: 2rem;
					}
					
					.dataTables_wrapper .dataTables_filter {
						float: right;
					}
					
					.dataTables_wrapper .dataTables_length select {
						background: #f8f9fa;
						border: 2px solid #e9ecef;
						border-radius: 8px;
						padding: 0.5rem 2rem 0.5rem 0.75rem;
						font-size: 0.9rem;
						margin: 0 0.5rem;
						transition: all 0.3s ease;
					}
					
					.dataTables_wrapper .dataTables_length select:focus {
						outline: none;
						border-color: #667eea;
						box-shadow: 0 0 0 3px rgba(102, 126, 234, 0.1);
					}
					
					.dataTables_wrapper .dataTables_filter input {
						background: #f8f9fa;
						border: 2px solid #e9ecef;
						border-radius: 8px;
						padding: 0.5rem 0.75rem;
						font-size: 0.9rem;
						margin-left: 0.5rem;
						transition: all 0.3s ease;
						min-width: 200px;
					}
					
					.dataTables_wrapper .dataTables_filter input:focus {
						outline: none;
						border-color: #667eea;
						background: white;
						box-shadow: 0 0 0 3px rgba(102, 126, 234, 0.1);
					}
					
					.dataTables_wrapper .dataTables_info,
					.dataTables_wrapper .dataTables_paginate {
						margin-top: 1.5rem;
						padding-top: 1rem;
						border-top: 1px solid #e9ecef;
					}
					
					.dataTables_wrapper .dataTables_info {
						float: left;
						color: #6c757d;
						font-size: 0.9rem;
						line-height: 2.5rem;
					}
					
					.dataTables_wrapper .dataTables_paginate {
						float: right;
					}
					
					.dataTables_wrapper .dataTables_paginate .paginate_button {
						background: #f8f9fa;
						border: 1px solid #dee2e6;
						color: #495057 !important;
						padding: 0.5rem 0.75rem;
						margin: 0 2px;
						border-radius: 6px;
						transition: all 0.3s ease;
						text-decoration: none !important;
					}
					
					.dataTables_wrapper .dataTables_paginate .paginate_button:hover {
						background: #667eea;
						border-color: #667eea;
						color: white !important;
						transform: translateY(-1px);
					}
					
					.dataTables_wrapper .dataTables_paginate .paginate_button.current {
						background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
						border-color: #667eea;
						color: white !important;
					}
					
					.dataTables_wrapper .dataTables_paginate .paginate_button.disabled {
						background: #e9ecef;
						border-color: #dee2e6;
						color: #6c757d !important;
						cursor: not-allowed;
						opacity: 0.6;
					}
					
					.dataTables_wrapper .dataTables_paginate .paginate_button.disabled:hover {
						background: #e9ecef;
						border-color: #dee2e6;
						color: #6c757d !important;
						transform: none;
					}
					
					/* Actions Column Visibility and Styling */
					.dataTables_wrapper table th:last-child,
					.dataTables_wrapper table td:last-child {
						min-width: 120px !important;
						width: 120px !important;
						max-width: 120px !important;
						text-align: center !important;
						position: sticky;
						right: 0;
						background: #fff;
						z-index: 10;
						border-left: 1px solid #dee2e6 !important;
					}
					
					/* Actions column header styling */
					.dataTables_wrapper table th:last-child {
						background: #f8f9fa !important;
						font-weight: 600;
						border-bottom: 2px solid #dee2e6 !important;
					}
					
					/* Row hover effect - maintain actions column background */
					.dataTables_wrapper table tbody tr:hover td:last-child {
						background: #f8f9fa !important;
					}
					
					/* Ensure actions column is always visible on mobile */
					@media (max-width: 768px) {
						.dataTables_wrapper table th:last-child,
						.dataTables_wrapper table td:last-child {
							min-width: 100px !important;
							width: 100px !important;
							max-width: 100px !important;
						}
					}
					
					.modern-table-wrapper {
						position: relative;
						overflow: hidden;
					}
					
					.table-loading-overlay {
						position: absolute;
						top: 0;
						left: 0;
						right: 0;
						bottom: 0;
						background: rgba(255,255,255,0.95);
						z-index: 1000;
						display: none;
						align-items: center;
						justify-content: center;
						backdrop-filter: blur(2px);
					}
					
					.loading-content {
						text-align: center;
						color: #6c757d;
					}
					
					.spinner-modern {
						width: 40px;
						height: 40px;
						border: 3px solid #f3f3f3;
						border-top: 3px solid #667eea;
						border-radius: 50%;
						animation: spin 1s linear infinite;
						margin: 0 auto 1rem;
					}
					
					@keyframes spin {
						0% { transform: rotate(0deg); }
						100% { transform: rotate(360deg); }
					}
					
					.loading-text {
						margin: 0;
						font-weight: 500;
					}
					
					.table-modern {
						width: 100%;
						margin: 0;
						background: white;
						border-collapse: separate;
						border-spacing: 0;
					}
					
					.table-header-modern {
						background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%);
					}
					
					.table-header-modern th {
						padding: 1rem 0.75rem;
						font-weight: 600;
						color: #495057;
						border-bottom: 2px solid #dee2e6;
						text-transform: uppercase;
						font-size: 0.85rem;
						letter-spacing: 0.5px;
					}
					
					.table-modern tbody tr {
						transition: all 0.2s ease;
						border-bottom: 1px solid #f8f9fa;
					}
					
					.table-modern tbody tr:hover {
						background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%);
						transform: translateY(-1px);
						box-shadow: 0 4px 8px rgba(0,0,0,0.05);
					}
					
					.table-modern tbody td {
						padding: 0.75rem;
						vertical-align: middle;
						border-bottom: 1px solid #f8f9fa;
					}
					
					/* Action Buttons */
					.action-buttons-modern {
						display: flex;
						gap: 0.5rem;
						justify-content: center;
					}
					
					.btn-action-edit, .btn-action-delete, .btn-action-files, .btn-action-search, .btn-action-search-title, .btn-action-metadata-refresh {
						background: none;
						border: none;
						padding: 0.5rem;
						border-radius: 8px;
						cursor: pointer;
						transition: all 0.3s ease;
						display: flex;
						align-items: center;
						justify-content: center;
						width: 36px;
						height: 36px;
					}

					.btn-action-edit {
						color: #0d6efd;
						background: rgba(13, 110, 253, 0.1);
					}

					.btn-action-edit:hover {
						background: rgba(13, 110, 253, 0.2);
						transform: translateY(-2px);
						box-shadow: 0 4px 12px rgba(13, 110, 253, 0.3);
					}

					.btn-action-delete {
						color: #dc3545;
						background: rgba(220, 53, 69, 0.1);
					}

					.btn-action-delete:hover {
						background: rgba(220, 53, 69, 0.2);
						transform: translateY(-2px);
						box-shadow: 0 4px 12px rgba(220, 53, 69, 0.3);
					}

					.btn-action-files {
						color: #198754;
						background: rgba(25, 135, 84, 0.1);
					}

					.btn-action-files:hover {
						background: rgba(25, 135, 84, 0.2);
						transform: translateY(-2px);
						box-shadow: 0 4px 12px rgba(25, 135, 84, 0.3);
					}

					.btn-action-search {
						color: #fd7e14;
						background: rgba(253, 126, 20, 0.1);
					}

					.btn-action-search:hover {
						background: rgba(253, 126, 20, 0.2);
						transform: translateY(-2px);
						box-shadow: 0 4px 12px rgba(253, 126, 20, 0.3);
					}

					.btn-action-search-title {
						color: #6f42c1;
						background: rgba(111, 66, 193, 0.1);
					}

					.btn-action-search-title:hover {
						background: rgba(111, 66, 193, 0.2);
						transform: translateY(-2px);
						box-shadow: 0 4px 12px rgba(111, 66, 193, 0.3);
					}

					.btn-action-metadata-refresh {
						color: #20c997;
						background: rgba(32, 201, 151, 0.1);
					}

					.btn-action-metadata-refresh:hover {
						background: rgba(32, 201, 151, 0.2);
						transform: translateY(-2px);
						box-shadow: 0 4px 12px rgba(32, 201, 151, 0.3);
					}

					.btn-action-edit i, .btn-action-delete i, .btn-action-files i, .btn-action-search i, .btn-action-search-title i, .btn-action-metadata-refresh i {
						font-size: 0.9rem;
					}

					/* ID Link Styling */
					.id-link {
						color: #0d6efd;
						text-decoration: none;
						font-weight: 500;
						transition: all 0.2s ease;
						padding: 2px 6px;
						border-radius: 4px;
					}

					.id-link:hover {
						color: #0a58ca;
						background: rgba(13, 110, 253, 0.1);
						text-decoration: underline;
					}

					.id-link:active {
						transform: scale(0.98);
					}

					/* Enhanced Status Messages */
					.alert-success-enhanced, .alert-danger-enhanced {
						margin: 1.5rem;
						padding: 1rem 1.5rem;
						border-radius: 12px;
						border: none;
						font-weight: 500;
						animation: slideDown 0.3s ease-out;
					}
					
					.alert-success-enhanced {
						background: linear-gradient(135deg, #d1edff 0%, #bfe3d0 100%);
						color: #0f5132;
						border-left: 4px solid #198754;
					}
					
					.alert-danger-enhanced {
						background: linear-gradient(135deg, #f8d7da 0%, #f5c2c7 100%);
						color: #842029;
						border-left: 4px solid #dc3545;
					}
					
					@keyframes slideDown {
						from {
							opacity: 0;
							transform: translateY(-10px);
						}
						to {
							opacity: 1;
							transform: translateY(0);
						}
					}
					
					/* Enhanced Modal Styling */
					.modal-enhanced .modal-dialog,
					#editFormModal .modal-dialog {
						max-width: 90% !important;
						width: 900px !important;
						margin: 1.5rem auto !important;
					}
					
					.modal-content-enhanced,
					#editFormModal .modal-content {
						border: none;
						border-radius: 16px;
						box-shadow: 0 20px 60px rgba(0,0,0,0.3);
						backdrop-filter: blur(10px);
						max-height: 90vh !important;
						display: flex !important;
						flex-direction: column !important;
					}
					
					.modal-header-enhanced,
					#editFormModal .modal-header {
						background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
						color: white;
						padding: 1.5rem 2rem;
						border-bottom: none;
						position: relative;
						flex-shrink: 0 !important;
					}
					
					.modal-header-enhanced::before {
						content: '';
						position: absolute;
						top: 0;
						left: 0;
						right: 0;
						bottom: 0;
						background: url('data:image/svg+xml,<svg width="60" height="60" viewBox="0 0 60 60" xmlns="http://www.w3.org/2000/svg"><g fill="none" fill-rule="evenodd"><g fill="%23ffffff" fill-opacity="0.05"><circle cx="30" cy="30" r="4"/></g></svg>');
						pointer-events: none;
					}
					
					.modal-title-section {
						display: flex;
						align-items: center;
						gap: 1rem;
						position: relative;
						z-index: 2;
					}
					
					.modal-icon {
						background: rgba(255,255,255,0.2);
						padding: 0.75rem;
						border-radius: 12px;
						font-size: 1.2rem;
						backdrop-filter: blur(10px);
					}
					
					.modal-title-text {
						margin: 0;
						font-size: 1.5rem;
						font-weight: 600;
						text-shadow: 0 2px 4px rgba(0,0,0,0.1);
					}
					
					.modal-close-btn {
						background: rgba(255,255,255,0.2);
						border: none;
						color: white;
						padding: 0.75rem;
						border-radius: 10px;
						font-size: 1.1rem;
						cursor: pointer;
						transition: all 0.3s ease;
						backdrop-filter: blur(10px);
						position: relative;
						z-index: 2;
					}
					
					.modal-close-btn:hover {
						background: rgba(255,255,255,0.3);
						transform: scale(1.05);
					}
					
					.modal-body-enhanced {
						padding: 2rem !important;
						background: #f8f9fa !important;
						max-height: 60vh !important;
						overflow-y: auto !important;
						min-height: 300px !important;
						flex: 1 1 auto !important;
						position: relative !important;
					}
					
					/* Ensure form content doesn't create additional scroll containers */
					.modal-body-enhanced .edit-form-fields,
					.modal-body-enhanced .config-form,
					.modal-body-enhanced .edit-form-container,
					.modal-body-enhanced .edit-form-modern,
					#editFormModal .modal-body .edit-form-fields,
					#editFormModal .modal-body .config-form,
					#editFormModal .modal-body .edit-form-container,
					#editFormModal .modal-body .edit-form-modern {
						overflow: visible !important;
						max-height: none !important;
						height: auto !important;
					}
					
					.modal-loading {
						position: absolute;
						top: 50%;
						left: 50%;
						transform: translate(-50%, -50%);
						text-align: center;
						z-index: 10;
					}
					
					.modal-footer-enhanced,
					#editFormModal .modal-footer {
						background: white;
						padding: 1.5rem 2rem;
						border-top: 1px solid #e9ecef;
						flex-shrink: 0 !important;
					}
					
					.modal-actions {
						display: flex;
						gap: 1rem;
						justify-content: flex-end;
					}
					
					.btn-modal-cancel, .btn-modal-save {
						padding: 0.75rem 1.5rem;
						border-radius: 10px;
						font-weight: 600;
						display: flex;
						align-items: center;
						gap: 0.5rem;
						transition: all 0.3s ease;
						border: none;
						cursor: pointer;
					}
					
					.btn-modal-cancel {
						background: #6c757d;
						color: white;
					}
					
					.btn-modal-cancel:hover {
						background: #5a6268;
						transform: translateY(-2px);
						box-shadow: 0 4px 12px rgba(108, 117, 125, 0.3);
					}
					
					.btn-modal-save {
						background: linear-gradient(135deg, #28a745 0%, #20c997 100%);
						color: white;
					}
					
					.btn-modal-save:hover {
						transform: translateY(-2px);
						box-shadow: 0 4px 12px rgba(40, 167, 69, 0.4);
					}
					
					/* Enhanced Page Styling */
					.config-section-enhanced {
						padding: 0;
						margin: 0;
					}
					
					.page-header-enhanced {
						background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
						color: white;
						padding: 1rem 0;
						margin-bottom: 2rem;
						position: relative;
						overflow: hidden;
					}
					
					.page-header-enhanced::before {
						content: '';
						position: absolute;
						top: 0;
						left: 0;
						right: 0;
						bottom: 0;
						background: url('data:image/svg+xml,<svg width="60" height="60" viewBox="0 0 60 60" xmlns="http://www.w3.org/2000/svg"><g fill="none" fill-rule="evenodd"><g fill="%23ffffff" fill-opacity="0.05"><circle cx="30" cy="30" r="4"/></g></svg>');
						animation: drift 20s infinite linear;
					}
					
					@keyframes drift {
						0% { transform: rotate(0deg) translate(-50%, -50%); }
						100% { transform: rotate(360deg) translate(-50%, -50%); }
					}
					
					.header-content {
						display: flex;
						align-items: center;
						gap: 2rem;
						max-width: 1200px;
						margin: 0 auto;
						padding: 0 2rem;
						position: relative;
						z-index: 2;
					}
					
					.header-icon-wrapper {
						background: rgba(255,255,255,0.2);
						padding: 1.5rem;
						border-radius: 20px;
						backdrop-filter: blur(10px);
						display: flex;
						align-items: center;
						justify-content: center;
					}
					
					.header-icon {
						font-size: 2.5rem;
						color: white;
					}
					
					.header-text {
						flex: 1;
					}
					
					.header-title {
						margin: 0 0 0.5rem 0;
						font-size: 2.5rem;
						font-weight: 700;
						text-shadow: 0 2px 10px rgba(0,0,0,0.3);
					}
					
					.header-subtitle {
						margin: 0;
						font-size: 1.1rem;
						opacity: 0.9;
						line-height: 1.6;
					}
					
					/* Enhanced Form Container */
					.form-container-enhanced {
						max-width: 1200px;
						margin: 0 auto;
						padding: 0 2rem;
					}
					
					.config-form-modern {
						background: transparent;
						border: none;
						padding: 0;
					}
					
					.form-cards-grid {
						display: grid;
						grid-template-columns: repeat(auto-fit, minmax(400px, 1fr));
						gap: 2rem;
						margin-bottom: 2rem;
					}
					
					.form-card {
						background: white;
						border-radius: 16px;
						box-shadow: 0 10px 30px rgba(0,0,0,0.1);
						overflow: hidden;
						transition: all 0.3s ease;
						border: 1px solid rgba(255,255,255,0.2);
					}
					
					.form-card:hover {
						transform: translateY(-5px);
						box-shadow: 0 20px 40px rgba(0,0,0,0.15);
					}
					
					.card-header {
						background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%);
						padding: 1.5rem 2rem;
						border-bottom: 1px solid #e9ecef;
						display: flex;
						align-items: center;
						gap: 1rem;
					}
					
					.card-icon {
						color: #667eea;
						font-size: 1.5rem;
					}
					
					.card-title {
						margin: 0 0 0.25rem 0;
						font-size: 1.25rem;
						font-weight: 600;
						color: #2c3e50;
					}
					
					.card-subtitle {
						margin: 0;
						font-size: 0.9rem;
						color: #6c757d;
					}
					
					.card-body {
						padding: 2rem;
					}
					
					/* Enhanced Action Buttons */
					.form-actions-enhanced {
						display: flex;
						gap: 1rem;
						justify-content: center;
						padding: 2rem;
						background: white;
						border-radius: 16px;
						box-shadow: 0 10px 30px rgba(0,0,0,0.1);
						margin-bottom: 2rem;
					}
					
					.btn-action-primary,
					.btn-action-secondary {
						display: flex;
						align-items: center;
						gap: 0.75rem;
						padding: 1rem 2rem;
						border-radius: 12px;
						font-weight: 600;
						font-size: 1rem;
						border: none;
						cursor: pointer;
						transition: all 0.3s ease;
						text-decoration: none;
					}
					
					.btn-action-primary {
						background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
						color: white;
						box-shadow: 0 4px 15px rgba(102, 126, 234, 0.4);
					}
					
					.btn-action-primary:hover {
						transform: translateY(-2px);
						box-shadow: 0 8px 25px rgba(102, 126, 234, 0.6);
					}
					
					.btn-action-secondary {
						background: #f8f9fa;
						color: #6c757d;
						border: 2px solid #e9ecef;
					}
					
					.btn-action-secondary:hover {
						background: #e9ecef;
						color: #495057;
						transform: translateY(-2px);
					}
					
					.action-icon {
						font-size: 1.1rem;
					}
					
					/* Enhanced Results Container */
					.results-container-enhanced {
						margin: 2rem auto;
						padding: 0 2rem;
						max-width: 1200px;
						min-height: 100px;
						border-radius: 16px;
						background: white;
						box-shadow: 0 10px 30px rgba(0,0,0,0.1);
					}
					
					/* Enhanced Help Section */
					.help-section-enhanced {
						max-width: 1200px;
						margin: 3rem auto 0;
						padding: 0 2rem;
					}
					
					.help-header {
						display: flex;
						align-items: center;
						gap: 1rem;
						margin-bottom: 2rem;
						text-align: center;
						justify-content: center;
					}
					
					.help-icon {
						color: #667eea;
						font-size: 1.5rem;
					}
					
					.help-title {
						margin: 0;
						font-size: 1.5rem;
						font-weight: 600;
						color: #2c3e50;
					}
					
					.help-content {
						background: white;
						border-radius: 16px;
						padding: 2rem;
						box-shadow: 0 10px 30px rgba(0,0,0,0.1);
					}
					
					.help-grid {
						display: grid;
						grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
						gap: 1.5rem;
						margin-bottom: 2rem;
					}
					
					.help-card {
						display: flex;
						align-items: flex-start;
						gap: 1rem;
						padding: 1.5rem;
						background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%);
						border-radius: 12px;
						border: 1px solid #e9ecef;
						transition: all 0.3s ease;
					}
					
					.help-card:hover {
						transform: translateY(-2px);
						box-shadow: 0 5px 15px rgba(0,0,0,0.1);
					}
					
					.help-card-icon {
						background: white;
						padding: 0.75rem;
						border-radius: 10px;
						box-shadow: 0 2px 10px rgba(0,0,0,0.1);
					}
					
					.help-card-icon i {
						color: #667eea;
						font-size: 1.25rem;
					}
					
					.help-card-content {
						flex: 1;
					}
					
					.help-card-content strong {
						display: block;
						font-size: 1rem;
						font-weight: 600;
						color: #2c3e50;
						margin-bottom: 0.25rem;
					}
					
					.help-card-content p {
						margin: 0;
						font-size: 0.9rem;
						color: #6c757d;
						line-height: 1.4;
					}
					
					.help-tips {
						border-top: 2px solid #e9ecef;
						padding-top: 2rem;
					}
					
					.tip-item {
						display: flex;
						align-items: flex-start;
						gap: 1rem;
						margin-bottom: 1rem;
						padding: 1rem;
						background: linear-gradient(135deg, #f8f9fa 0%, #ffffff 100%);
						border-radius: 10px;
						border-left: 4px solid #667eea;
					}
					
					.tip-icon {
						color: #667eea;
						font-size: 1rem;
						margin-top: 0.1rem;
					}
					
					.tip-item strong {
						color: #2c3e50;
					}
					
					/* Mobile Responsiveness */
					@media (max-width: 768px) {
						.header-content {
							flex-direction: column;
							text-align: center;
							gap: 1.5rem;
						}
						
						.header-title {
							font-size: 2rem;
						}
						
						.form-cards-grid {
							grid-template-columns: 1fr;
						}
						
						.form-actions-enhanced {
							flex-direction: column;
							align-items: center;
						}
						
						.help-grid {
							grid-template-columns: 1fr;
						}
						
						.help-card {
							flex-direction: column;
							text-align: center;
						}
					}

					/* Enhanced Form Styling */
					.edit-form-container {
						padding: 2rem;
						background: white;
					}
					
					.edit-form-header {
						margin-bottom: 2rem;
						text-align: center;
						padding-bottom: 1rem;
						border-bottom: 2px solid #e9ecef;
					}
					
					.edit-form-title {
						margin: 0 0 0.5rem 0;
						font-size: 1.5rem;
						font-weight: 700;
						color: #495057;
					}
					
					.edit-form-subtitle {
						margin: 0;
						color: #6c757d;
						font-size: 0.95rem;
					}
					
					.edit-form-modern {
						max-height: none !important;
						overflow-y: visible !important;
						overflow: visible !important;
						padding-right: 0.5rem;
					}
					
					.edit-form-fields {
						display: grid;
						grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
						gap: 1.5rem;
					}
					
					.form-field-enhanced {
						position: relative;
						background: white;
						border-radius: 12px;
						padding: 1rem;
						border: 1px solid #e9ecef;
						transition: all 0.3s ease;
					}
					
					.form-field-enhanced:hover {
						border-color: #667eea;
						box-shadow: 0 4px 12px rgba(102, 126, 234, 0.1);
						transform: translateY(-2px);
					}
					
					.form-label-modern {
						display: flex;
						align-items: center;
						gap: 0.5rem;
						font-weight: 600;
						color: #495057;
						margin-bottom: 0.75rem;
						font-size: 0.9rem;
						text-transform: uppercase;
						letter-spacing: 0.5px;
					}
					
					.field-icon {
						background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
						color: white;
						padding: 0.4rem;
						border-radius: 6px;
						font-size: 0.8rem;
					}
					
					.form-control-modern {
						width: 100%;
						padding: 0.75rem;
						border: 2px solid #e9ecef;
						border-radius: 8px;
						font-size: 0.95rem;
						transition: all 0.3s ease;
						background: #f8f9fa;
					}
					
					.form-control-modern:focus {
						outline: none;
						border-color: #667eea;
						background: white;
						box-shadow: 0 0 0 3px rgba(102, 126, 234, 0.1);
					}
					
					/* Choices.js styling to match form-control-modern */
					.choices[data-type*="select-one"] .choices__inner,
					.choices[data-type*="select-multiple"] .choices__inner {
						width: 100%;
						padding: 0.75rem;
						border: 2px solid #e9ecef;
						border-radius: 8px;
						font-size: 0.95rem;
						transition: all 0.3s ease;
						background: #f8f9fa;
						min-height: auto;
					}
					
					.choices[data-type*="select-one"]:not(.is-disabled) .choices__inner,
					.choices[data-type*="select-multiple"]:not(.is-disabled) .choices__inner {
						cursor: pointer;
					}
					
					.choices.is-focused .choices__inner,
					.choices.is-open .choices__inner {
						border-color: #667eea;
						background: white;
						box-shadow: 0 0 0 3px rgba(102, 126, 234, 0.1);
					}
					
					.choices__list--single .choices__item {
						padding: 0;
						font-size: 0.95rem;
						color: #495057;
					}
					
					.choices__input {
						padding: 0;
						margin: 0;
						font-size: 0.95rem;
						background: transparent;
						color: #495057;
					}
					
					.choices__input:focus {
						outline: none;
					}
					
					.choices__list--dropdown {
						border: 2px solid #667eea;
						border-radius: 8px;
						box-shadow: 0 4px 12px rgba(102, 126, 234, 0.15);
						background: white;
						margin-top: 4px;
					}
					
					.choices__list--dropdown .choices__item {
						padding: 0.75rem;
						font-size: 0.95rem;
						border-bottom: 1px solid #f8f9fa;
						transition: all 0.2s ease;
					}
					
					.choices__list--dropdown .choices__item:last-child {
						border-bottom: none;
					}
					
					.choices__item--choice.is-highlighted {
						background: #667eea;
						color: white;
					}
					
					.choices__item--choice:hover {
						background: #f8f9fa;
					}
					
					.choices__item--choice.is-highlighted:hover {
						background: #667eea;
					}
					
					.choices__placeholder {
						color: #6c757d;
						font-style: italic;
					}
					
					/* Remove item button styling */
					.choices__button {
						background: none;
						border: none;
						color: #6c757d;
						padding: 0 0.5rem;
						font-size: 1.2rem;
						transition: color 0.2s ease;
					}
					
					.choices__button:hover {
						color: #dc3545;
					}
					
					.form-switch-enhanced {
						display: flex;
						align-items: center;
						justify-content: space-between;
						padding: 1rem;
						background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%);
						border-radius: 12px;
						border: 1px solid #dee2e6;
					}
					
					.form-check-input-modern {
						width: 3rem;
						height: 1.5rem;
						background: #dee2e6;
						border: none;
						border-radius: 1rem;
						position: relative;
						cursor: pointer;
						transition: all 0.3s ease;
						appearance: none;
    					margin-right: 15px;
						margin-left: 15px;
					}
					
					.form-check-input-modern:checked {
						background: linear-gradient(135deg, #28a745 0%, #20c997 100%);
					}
					
					.form-check-input-modern::before {
						content: '';
						position: absolute;
						top: 2px;
						left: 2px;
						width: 1.25rem;
						height: 1.25rem;
						background: white;
						border-radius: 50%;
						transition: all 0.3s ease;
						box-shadow: 0 2px 4px rgba(0,0,0,0.2);
					}
					
					.form-check-input-modern:checked::before {
						transform: translateX(1.5rem);
					}
					
					/* Responsive Design */
					@media (max-width: 768px) {
						.database-header-content {
							flex-direction: column;
							gap: 1.5rem;
							text-align: center;
						}
						
						.database-actions {
							width: 100%;
							justify-content: center;
						}
						
						.filters-grid {
							grid-template-columns: 1fr;
						}
						
						.action-buttons-modern {
							flex-direction: column;
						}
						
						.database-content-container {
							margin: 1rem;
						}
						
						.database-table-container {
							margin: 1rem;
						}
						
						.filters-card-enhanced {
							margin: 1rem;
						}
						
						.edit-form-fields {
							grid-template-columns: 1fr;
						}
						
						.modal-enhanced .modal-dialog,
						#editFormModal .modal-dialog {
							margin: 1rem !important;
							max-width: calc(100% - 2rem) !important;
							width: auto !important;
						}
						
						.modal-actions {
							flex-direction: column;
						}
					}
				`))
}

func page(
	_ string,
	activeConfig bool,
	activeDatabase bool,
	activeManagement bool,
	addcontent ...gomponents.Node,
) gomponents.Node {
	return html.Doctype(
		html.HTML(
			html.Lang("en"),
			html.Head(
				html.Meta(html.Charset("utf-8")),
				html.Meta(
					html.Name("viewport"),
					html.Content("width=device-width, initial-scale=1"),
				),
				html.Title("Media Downloader Management"),

				// Load jQuery first
				html.Script(html.Src("https://code.jquery.com/jquery-3.7.1.min.js")),
				html.Link(
					html.Rel("stylesheet"),
					html.Href("/static/css/light.css"),
				), // https://cdn.jsdelivr.net/npm/@adminkit/core@3.4.0/dist/css/app.min.css
				// Choices.js CSS and JS for searchable dropdowns
				// html.Link(html.Rel("stylesheet"), html.Href("https://cdn.jsdelivr.net/npm/choices.js/public/assets/styles/choices.min.css")),
				html.Script(html.Src("/static/js/app.js")),
				html.Script(html.Src("/static/js/datatables.js")),
				// html.Script(html.Src("https://cdn.jsdelivr.net/npm/choices.js/public/assets/scripts/choices.min.js")),

				html.Script(html.Src("https://unpkg.com/htmx.org")),
				addCSS(),
				// adminStyles(),
			),
			html.Body(
				html.Div(html.Class("wrapper"),
					createNavbar(activeConfig, activeDatabase, activeManagement),
					html.Div(html.Class("main"),
						html.Nav(html.Class("navbar navbar-expand navbar-light navbar-bg"),
							html.A(html.Class("sidebar-toggle js-sidebar-toggle"),
								html.I(html.Class("hamburger align-self-center")),
							),
						),
						html.Main(html.Class("content"),
							html.Div(html.Class("container-fluid p-0"),
								// html.H1(html.Class("h3 mb-3"), gomponents.Text(headertext)),
								html.Div(
									append([]gomponents.Node{html.Class("row")}, addcontent...)...),
							),
						),
						html.Footer(
							html.Class("content-footer"),
							// System Status Footer
							html.Div(
								html.Class("bg-dark text-white py-4"),
								html.Div(
									html.Class("container"),
									html.Div(
										html.Class("row align-items-center"),
										html.Div(
											html.Class("col-md-6"),
											html.P(
												html.Class("mb-0"),
												gomponents.Text(
													"© 2025 Go Media Downloader - Advanced Media Automation",
												),
											),
										),
										html.Div(
											html.Class("col-md-6 text-md-end"),
											html.Span(
												html.Class("badge bg-success me-2"),
												html.I(html.Class("fas fa-circle me-1")),
												gomponents.Text("System Online"),
											),
											html.Small(
												html.Class("text-muted"),
												gomponents.Text("Web Interface v2.0"),
											),
										),
									),
								),
							),
						),
					),
				),

				// Toast notification container
				html.Div(
					html.Class("toast-container position-fixed top-0 end-0 p-3"),
					html.ID("toastContainer"),
					html.Style("z-index: 1055;"),
				),

				adminJavaScript(),
			),
		),
	)
}

func adminPage() string {
	pageNode := page("Go Media Downloader", false, false, false, renderModernAdminIntro())

	// Render to string
	var buf strings.Builder
	pageNode.Render(&buf)

	return buf.String()
}

// adminPage generates the HTML page using gomponents
// adminPageConfig - consolidated handler for all config pages.
func adminPageConfig(ctx *gin.Context) {
	configType, ok := getParamID(ctx, "configtype")
	if !ok {
		return
	}

	var pageNode gomponents.Node

	csrfToken := getCSRFToken(ctx)

	switch configType {
	case "general":
		configv := config.GetSettingsGeneral()

		pageNode = page(
			"Config General",
			true,
			false,
			false,
			renderGeneralConfig(configv, csrfToken),
		)

	case "imdb":
		configv := config.GetSettingsImdb()

		pageNode = page("Config Imdb", true, false, false, renderImdbConfig(configv, csrfToken))

	case "media":
		configv := config.GetSettingsMediaAll()

		pageNode = page("Config Media", true, false, false, renderMediaConfig(configv, csrfToken))

	case "downloader":
		configv := config.GetSettingsDownloaderAll()

		pageNode = page(
			"Config Downloader",
			true,
			false,
			false,
			renderDownloaderConfig(configv, csrfToken),
		)

	case "indexers":
		configv := config.GetSettingsIndexerAll()

		pageNode = page(
			"Config Indexer",
			true,
			false,
			false,
			renderIndexersConfig(configv, csrfToken),
		)

	case "lists":
		configv := config.GetSettingsListAll()

		pageNode = page("Config Lists", true, false, false, renderListsConfig(configv, csrfToken))

	case "paths":
		configv := config.GetSettingsPathAll()

		pageNode = page("Config Paths", true, false, false, renderPathsConfig(configv, csrfToken))

	case "notifications":
		configv := config.GetSettingsNotificationAll()

		pageNode = page(
			"Config Notifications",
			true,
			false,
			false,
			renderNotificationConfig(configv, csrfToken),
		)

	case "quality":
		configv := config.GetSettingsQualityAll()

		pageNode = page(
			"Config Quality",
			true,
			false,
			false,
			renderQualityConfig(configv, csrfToken),
		)

	case "regex":
		configv := config.GetSettingsRegexAll()

		pageNode = page("Config Regex", true, false, false, renderRegexConfig(configv, csrfToken))

	case "scheduler":
		configv := config.GetSettingsSchedulerAll()

		pageNode = page(
			"Config Scheduler",
			true,
			false,
			false,
			renderSchedulerConfig(configv, csrfToken),
		)

	default:
		sendNotFound(ctx, "unknown config type: "+configType)
		return
	}

	// Render to string
	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageTestParse(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page("String Parse Test", false, false, true, renderTestParsePage(csrfToken))

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageMovieMetadata(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page(
		"Movie Metadata Lookup",
		false,
		false,
		true,
		renderMovieMetadataPage(csrfToken),
	)

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageTraktAuth(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page("Trakt Authentication", false, false, true, renderTraktAuthPage(csrfToken))

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageNamingTest(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page("Naming Convention Test", false, false, true, renderNamingTestPage(csrfToken))

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageJobManagement(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page("Job Management", false, false, true, renderJobManagementPage(csrfToken))

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageCronGenerator(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page(
		"Cron Expression Generator",
		false,
		false,
		true,
		renderCronGeneratorPage(csrfToken),
	)

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageDebugStats(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page("Debug Statistics", false, false, true, renderDebugStatsPage(csrfToken))

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageDatabaseMaintenance(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page(
		"Database Maintenance",
		false,
		false,
		true,
		renderDatabaseMaintenancePage(csrfToken),
	)

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageSearchDownload(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page("Search & Download", false, false, true, renderSearchDownloadPage(csrfToken))

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPagePushoverTest(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page("Pushover Test", false, false, true, renderPushoverTestPage(csrfToken))

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageLogViewer(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page("Log Viewer", false, false, true, renderLogViewerPage(csrfToken))

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageFeedParsing(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page("Feed Parser & Results", false, false, true, renderFeedParsingPage(csrfToken))

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageFolderStructure(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page(
		"Folder Structure Organizer",
		false,
		false,
		true,
		renderFolderStructurePage(csrfToken),
	)

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

// New Management Tools Admin Page Handlers.
func adminPageMediaCleanup(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page(
		"Media Cleanup Wizard",
		false,
		false,
		true,
		renderMediaCleanupWizardPage(csrfToken),
	)

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageMissingEpisodes(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page(
		"Missing Episodes Finder",
		false,
		false,
		true,
		renderMissingEpisodesFinderPage(csrfToken),
	)

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageLogAnalysis(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page(
		"Log Analysis Dashboard",
		false,
		false,
		true,
		renderLogAnalysisDashboardPage(csrfToken),
	)

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageStorageHealth(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page(
		"Storage Health Monitor",
		false,
		false,
		true,
		renderStorageHealthMonitorPage(csrfToken),
	)

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageServiceHealth(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page(
		"External Service Health Check",
		false,
		false,
		true,
		renderExternalServiceHealthCheckPage(csrfToken),
	)

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageAPITesting(ctx *gin.Context) {
	// csrfToken := getCSRFToken(ctx)
	pageNode := page("API Testing Suite", false, false, true, renderAPITestingPage())

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageGrid(ctx *gin.Context) {
	grid, ok := getParamID(ctx, "grid")
	if !ok {
		return
	}

	switch grid {
	case "queue":
		renderQueuePage(ctx)
	case "scheduler":
		renderSchedulerPage(ctx)
	case "stats":
		renderStatsPage(ctx)
	}
}

// adminPageDatabase - consolidated handler for all database table pages.
func adminPageDatabase(ctx *gin.Context) {
	tableName, ok := getParamID(ctx, "tablename")
	if !ok {
		return
	}

	// Reject invalid table names (like source maps, static files, etc.)
	if strings.Contains(tableName, ".") || strings.Contains(tableName, "/") {
		sendNotFound(ctx, "Invalid table name: "+tableName)
		return
	}

	// Verify the table exists by checking if GetTableDefaults returns valid data
	tableDefault := database.GetTableDefaults(tableName)
	if tableDefault.Table == "" {
		sendNotFound(ctx, "Table not found: "+tableName)
		return
	}

	// Create page title from table name
	pageTitle := "Database " + strings.ToTitle(strings.ReplaceAll(tableName, "_", " "))

	pageNode := page(
		pageTitle,
		false,
		true,
		false,
		adminDatabaseContent(tableName, getCSRFToken(ctx)),
	)
	// Render to string
	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func AdminPageAny(ctx *gin.Context, pageTitle string, content gomponents.Node) {
	pageNode := page(pageTitle, false, false, true, content)
	// Render to string
	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageQualityReorder(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page(
		"Quality Reorder Testing",
		false,
		false,
		true,
		renderQualityReorderPage(csrfToken),
	)

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageRegexTester(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page("Regex Pattern Tester", false, false, true, renderRegexTesterPage(csrfToken))

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageNamingGenerator(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page(
		"Naming Template Generator",
		false,
		false,
		true,
		renderNamingGeneratorPage(csrfToken),
	)

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}
